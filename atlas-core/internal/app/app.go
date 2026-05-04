package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/function"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/postgres"
)

type App struct {
	Config *config.Config
	Logger *logging.Logger
	Funcs  function.Functions

	pool            *pgxpool.Pool
	stores          stores
	objStore        *objectstorage.Store
	reconcileCancel context.CancelFunc
}

type stores struct {
	Entity      *postgres.EntityStore
	Object      *postgres.ObjectStore
	Task        *postgres.TaskStore
	Observation *postgres.ObservationStore
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	runID := os.Getenv("ATLAS_RUN_ID")
	if runID == "" {
		runID = "local"
	}

	log := logging.New(cfg, runID)
	log.Info("app", "Atlas Core starting")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := postgres.InitSchema(ctx, pool, log); err != nil {
		pool.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	objStore := objectstorage.NewStore(cfg.ObjectStorageDir, log)
	if err := objStore.InitRoot(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("init object storage: %w", err)
	}

	entityStore := postgres.NewEntityStore(pool, log)
	objectStore := postgres.NewObjectStore(pool, log)
	taskStore := postgres.NewTaskStore(pool, log)
	observationStore := postgres.NewObservationStore(pool, log)

	funcs := function.Functions{
		Entity:      function.NewEntityFunctions(entityStore, log),
		Object:      function.NewObjectFunctions(objectStore, objStore, log),
		Task:        function.NewTaskFunctions(taskStore, objectStore, log),
		Observation: function.NewObservationFunctions(observationStore, log),
	}

	app := &App{
		Config: cfg,
		Logger: log,
		pool:   pool,
		stores: stores{
			Entity:      entityStore,
			Object:      objectStore,
			Task:        taskStore,
			Observation: observationStore,
		},
		Funcs:    funcs,
		objStore: objStore,
	}

	if err := app.Funcs.Object.Reconcile(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reconcile object state: %w", err)
	}
	if err := app.markReady(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("mark app ready: %w", err)
	}
	app.startReconciler()

	log.Info("app", "Atlas Core started successfully")
	return app, nil
}

func (a *App) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	a.Logger.Info("app", "received signal, shutting down", logging.String("signal", sig.String()))
	a.Shutdown()
}

func (a *App) Shutdown() {
	a.Logger.Info("app", "shutting down Atlas Core")
	if a.reconcileCancel != nil {
		a.reconcileCancel()
	}
	if a.pool != nil {
		a.pool.Close()
	}
	if err := os.Remove(a.Config.ReadyFile); err != nil && !os.IsNotExist(err) {
		a.Logger.Warn("app", "failed to remove ready file", logging.String("ready_file", a.Config.ReadyFile), logging.ErrorField(err))
	}
	a.Logger.Info("app", "Atlas Core stopped")
}

func (a *App) markReady() error {
	if err := os.MkdirAll(filepath.Dir(a.Config.ReadyFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.Config.ReadyFile, []byte("ready\n"), 0o644)
}

func (a *App) startReconciler() {
	if a.Config.ReconcileInterval <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.reconcileCancel = cancel
	go func() {
		ticker := time.NewTicker(a.Config.ReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, runCancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := a.Funcs.Object.Reconcile(runCtx); err != nil {
					a.Logger.ErrorContext(runCtx, "object_reconcile", "periodic reconciliation failed", logging.ErrorField(err))
				}
				runCancel()
			}
		}
	}()
}
