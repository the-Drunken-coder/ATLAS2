package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	"github.com/anomalyco/atlas-core/services/fusion/engines"
	"github.com/anomalyco/atlas-core/services/fusion/internal/atlasio"
	fusionruntime "github.com/anomalyco/atlas-core/services/fusion/runtime"
	"github.com/anomalyco/atlas-core/services/shared/config"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		readyFile := os.Getenv("ATLAS_READY_FILE")
		if readyFile == "" {
			readyFile = "/var/lib/atlas-fusion/.ready"
		}
		if _, err := os.Stat(readyFile); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.LoadFusion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	runID := os.Getenv("ATLAS_RUN_ID")
	if runID == "" {
		runID = "local"
	}
	log := logging.New(cfg.LogLevel, "atlas-fusion", runID)
	if err := removeReadyFile(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(cfg.FunctionsAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "functions dial error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()
	if err := waitForClientReady(dialCtx, conn); err != nil {
		fmt.Fprintf(os.Stderr, "functions dial error: %v\n", err)
		os.Exit(1)
	}

	client := functionsv1.NewAtlasFunctionsServiceClient(conn)
	runner := fusionruntime.Runner{
		Source:          atlasio.Source{Client: client},
		Sink:            atlasio.Sink{Client: client},
		CheckpointStore: fusionruntime.FileCheckpointStore{Path: cfg.CheckpointFile},
		PageSize:        cfg.PageSize,
	}
	if cfg.EnableReferenceEngine {
		runner.Engines = []core.Engine{engines.ReferenceEngine{}}
	}
	if err := markReady(cfg.ReadyFile); err != nil {
		fmt.Fprintf(os.Stderr, "ready error: %v\n", err)
		os.Exit(1)
	}
	log.Info("main", "atlas fusion running",
		logging.String("functions_addr", cfg.FunctionsAddress),
		logging.String("checkpoint_file", cfg.CheckpointFile),
		logging.Any("reference_engine_enabled", cfg.EnableReferenceEngine),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.EnableReferenceEngine {
		runLoop(ctx, runner, cfg.PollInterval, log)
	} else {
		log.Info("main", "no fusion engine registered; worker is idle")
		<-ctx.Done()
	}
	log.Info("main", "shutting down atlas fusion")
	if err := os.Remove(cfg.ReadyFile); err != nil && !os.IsNotExist(err) {
		log.Warn("main", "failed to remove ready file", logging.ErrorField(err))
	}
}

func runLoop(ctx context.Context, runner fusionruntime.Runner, interval time.Duration, log *logging.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		stats, err := runner.RunOnce(ctx)
		if err != nil {
			log.Warn("runtime", "fusion iteration failed", logging.ErrorField(err))
		} else {
			log.Info("runtime", "fusion iteration completed",
				logging.Any("observations", stats.ObservationCount),
				logging.Any("track_updates", stats.TrackUpdateCount),
				logging.Any("provenance", stats.ProvenanceCount),
				logging.String("checkpoint_observation_id", stats.Checkpoint.ObservationID),
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
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

func waitForClientReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return fmt.Errorf("connection shut down")
		}
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}
