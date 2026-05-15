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

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	"github.com/anomalyco/atlas-core/services/functions/internal/datastorageclient"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	functionsservice "github.com/anomalyco/atlas-core/services/functions/internal/service"
	"github.com/anomalyco/atlas-core/services/shared/config"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		readyFile := os.Getenv("ATLAS_READY_FILE")
		if readyFile == "" {
			readyFile = "/var/lib/atlas-functions/.ready"
		}
		if _, err := os.Stat(readyFile); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.LoadFunctions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	runID := os.Getenv("ATLAS_RUN_ID")
	if runID == "" {
		runID = "local"
	}
	log := logging.New(cfg.LogLevel, "atlas-functions", runID)
	if err := removeReadyFile(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, cfg.DataStorageAddress, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		fmt.Fprintf(os.Stderr, "datastorage dial error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	validator, err := protocolvalidation.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "validator error: %v\n", err)
		os.Exit(1)
	}
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(conn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, log, validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, log, validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, log, validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, log, validator, hub),
	}

	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	functionsservice.RegisterGRPC(grpcServer, funcs, hub)

	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcServer.Serve(listener) }()
	if err := markReady(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "ready error: %v\n", err)
		os.Exit(1)
	}
	log.Info("main", "atlas functions running", logging.String("listen_addr", cfg.ListenAddress), logging.String("datastorage_addr", cfg.DataStorageAddress))

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
		log.Info("main", "shutting down atlas functions", logging.String("signal", sig.String()))
	}

	stopGRPCServer(grpcServer, 5*time.Second)
	if err := os.Remove(cfg.ReadyFile); err != nil && !os.IsNotExist(err) {
		log.Warn("main", "failed to remove ready file", logging.ErrorField(err))
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
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
