package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/service"
	"github.com/anomalyco/atlas-core/services/shared/config"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		if _, err := os.Stat(readyFileForHealthcheck()); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.LoadDataStorage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	runID := os.Getenv("ATLAS_RUN_ID")
	if runID == "" {
		runID = "local"
	}
	log := logging.New(cfg.LogLevel, "atlas-datastorage", runID)
	if err := removeReadyFile(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ReconcileTimeout)
	defer cancel()
	svc, err := service.New(ctx, cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(service.InternalAuthUnaryInterceptor(cfg.InternalToken)),
		grpc.StreamInterceptor(service.InternalAuthStreamInterceptor(cfg.InternalToken)),
	)
	service.RegisterGRPC(grpcServer, svc)

	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcServer.Serve(listener) }()
	if err := markReady(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "ready error: %v\n", err)
		os.Exit(1)
	}
	svc.StartReconciler()
	log.Info("main", "atlas datastorage running", logging.String("listen_addr", cfg.ListenAddress))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	exitCode := 0
	select {
	case err := <-serveErr:
		if err != nil {
			fmt.Fprintf(os.Stderr, "grpc serve error: %v\n", err)
			exitCode = 1
		}
	case sig := <-sigCh:
		log.Info("main", "shutting down atlas datastorage", logging.String("signal", sig.String()))
	}

	stopGRPCServer(grpcServer, 5*time.Second)
	if err := os.Remove(cfg.ReadyFile); err != nil && !os.IsNotExist(err) {
		log.Warn("main", "failed to remove ready file", logging.ErrorField(err))
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func readyFileForHealthcheck() string {
	cfg, err := config.LoadDataStorage()
	if err == nil {
		return cfg.ReadyFile
	}
	readyFile := os.Getenv("ATLAS_READY_FILE")
	if readyFile == "" {
		readyFile = "/var/lib/atlas-datastorage/.ready"
	}
	return readyFile
}

func removeReadyFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func markReady(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("ready\n"), 0o644)
}

func stopGRPCServer(server *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		server.Stop()
		<-done
	}
}
