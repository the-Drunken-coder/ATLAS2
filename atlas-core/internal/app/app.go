package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/function"
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/objectstorage"
	"github.com/anomalyco/atlas-core/internal/postgres"
)

type App struct {
	Config   *config.Config
	Logger   *logging.Logger
	Pool     interface{ Close() }
	Stores   Stores
	Funcs    function.Functions
	ObjStore *objectstorage.Store
}

type Stores struct {
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

	entityStore := postgres.NewEntityStore(pool)
	objectStore := postgres.NewObjectStore(pool)
	taskStore := postgres.NewTaskStore(pool)
	observationStore := postgres.NewObservationStore(pool)

	funcs := function.Functions{
		Entity:      function.NewEntityFunctions(entityStore, log),
		Object:      function.NewObjectFunctions(objectStore, objStore, log),
		Task:        function.NewTaskFunctions(taskStore, log),
		Observation: function.NewObservationFunctions(observationStore, log),
	}

	app := &App{
		Config: cfg,
		Logger: log,
		Pool:   pool,
		Stores: Stores{
			Entity:      entityStore,
			Object:      objectStore,
			Task:        taskStore,
			Observation: observationStore,
		},
		Funcs:    funcs,
		ObjStore: objStore,
	}

	log.Info("app", "Atlas Core started successfully")
	return app, nil
}

func (a *App) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	a.Logger.Info("app", fmt.Sprintf("received signal %s, shutting down", sig))
	a.Shutdown()
}

func (a *App) Shutdown() {
	a.Logger.Info("app", "shutting down Atlas Core")
	if pool, ok := a.Pool.(interface{ Close() }); ok {
		pool.Close()
	}
	a.Logger.Info("app", "Atlas Core stopped")
}
