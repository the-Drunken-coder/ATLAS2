package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/function"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/postgres"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
)

type App struct {
	Config *config.Config
	Logger *logging.Logger
	Funcs  function.Functions

	pool            *pgxpool.Pool
	stores          stores
	objStore        *objectstorage.Store
	reconcileCancel context.CancelFunc
	reconcileWG     sync.WaitGroup
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

	if err := removeReadyFile(cfg.ReadyFile); err != nil {
		return nil, fmt.Errorf("reset ready file: %w", err)
	}

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
	idemStore := postgres.NewIdempotencyStore(pool, log)

	protoValidator, err := protocolvalidation.New()
	if err != nil {
		closeStartupResources(pool, objStore, log)
		return nil, fmt.Errorf("init protocol validator: %w", err)
	}

	funcs := function.Functions{
		Entity:      function.NewEntityFunctions(entityStore, log, protoValidator),
		Object:      function.NewObjectFunctions(objectStore, objStore, idemStore, log, protoValidator),
		Task:        function.NewTaskFunctions(taskStore, objectStore, entityStore, idemStore, log, protoValidator),
		Observation: function.NewObservationFunctions(observationStore, log, protoValidator),
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

	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), cfg.ReconcileTimeout)
	defer reconcileCancel()
	if err := app.Funcs.Object.Reconcile(reconcileCtx); err != nil {
		closeStartupResources(pool, objStore, log)
		return nil, fmt.Errorf("reconcile object state: %w", err)
	}
	if err := app.markReady(); err != nil {
		closeStartupResources(pool, objStore, log)
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
	a.reconcileWG.Wait()
	if a.pool != nil {
		a.pool.Close()
	}
	if a.objStore != nil {
		if err := a.objStore.Close(); err != nil {
			a.Logger.Warn("app", "failed to close object storage", logging.ErrorField(err))
		}
	}
	if err := os.Remove(a.Config.ReadyFile); err != nil && !os.IsNotExist(err) {
		a.Logger.Warn("app", "failed to remove ready file", logging.String("ready_file", a.Config.ReadyFile), logging.ErrorField(err))
	}
	a.Logger.Info("app", "Atlas Core stopped")
}

func (a *App) markReady() error {
	if err := os.MkdirAll(filepath.Dir(a.Config.ReadyFile), 0o700); err != nil {
		return err
	}
	return os.WriteFile(a.Config.ReadyFile, []byte("ready\n"), 0o644)
}

func removeReadyFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *App) startReconciler() {
	if a.Config.ReconcileInterval <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.reconcileCancel = cancel
	a.reconcileWG.Add(1)
	go func() {
		defer a.reconcileWG.Done()
		ticker := time.NewTicker(a.Config.ReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, runCancel := context.WithTimeout(ctx, a.Config.ReconcileTimeout)
				if err := a.Funcs.Object.Reconcile(runCtx); err != nil && ctx.Err() == nil {
					a.Logger.ErrorContext(runCtx, "object_reconcile", "periodic reconciliation failed", logging.ErrorField(err))
				}
				runCancel()
			}
		}
	}()
}

func closeStartupResources(pool *pgxpool.Pool, objStore *objectstorage.Store, log *logging.Logger) {
	if objStore != nil {
		if err := objStore.Close(); err != nil {
			log.Warn("app", "failed to close object storage during startup cleanup", logging.ErrorField(err))
		}
	}
	if pool != nil {
		pool.Close()
	}
}
