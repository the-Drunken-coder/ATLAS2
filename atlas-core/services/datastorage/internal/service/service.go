package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/datastorage/internal/objectstorage"
	"github.com/anomalyco/atlas-core/services/datastorage/internal/postgres"
	"github.com/anomalyco/atlas-core/services/shared/config"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type Service struct {
	Config *config.DataStorageConfig
	Logger *logging.Logger

	pool             *pgxpool.Pool
	entityStore      *postgres.EntityStore
	objectStore      *postgres.ObjectStore
	taskStore        *postgres.TaskStore
	observationStore *postgres.ObservationStore
	idempotencyStore *postgres.IdempotencyStore
	objectStorage    *objectstorage.Store
	reconcileCancel  context.CancelFunc
	reconcileWG      sync.WaitGroup
}

func New(ctx context.Context, cfg *config.DataStorageConfig, log *logging.Logger) (*Service, error) {
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
	svc := &Service{
		Config:           cfg,
		Logger:           log,
		pool:             pool,
		entityStore:      postgres.NewEntityStore(pool, log),
		objectStore:      postgres.NewObjectStore(pool, log),
		taskStore:        postgres.NewTaskStore(pool, log),
		observationStore: postgres.NewObservationStore(pool, log),
		idempotencyStore: postgres.NewIdempotencyStore(pool, log),
		objectStorage:    objStore,
	}
	reconcileCtx, cancel := context.WithTimeout(ctx, cfg.ReconcileTimeout)
	defer cancel()
	if err := svc.ReconcileObjects(reconcileCtx); err != nil {
		svc.Close()
		return nil, fmt.Errorf("reconcile object state: %w", err)
	}
	return svc, nil
}

func (s *Service) StartReconciler() {
	if s.Config.ReconcileInterval <= 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.reconcileCancel = cancel
	s.reconcileWG.Add(1)
	go func() {
		defer s.reconcileWG.Done()
		ticker := time.NewTicker(s.Config.ReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, runCancel := context.WithTimeout(ctx, s.Config.ReconcileTimeout)
				if err := s.ReconcileObjects(runCtx); err != nil && ctx.Err() == nil {
					s.Logger.ErrorContext(runCtx, "object_reconcile", "periodic reconciliation failed", logging.ErrorField(err))
				}
				runCancel()
			}
		}
	}()
}

func (s *Service) Close() {
	if s.reconcileCancel != nil {
		s.reconcileCancel()
	}
	s.reconcileWG.Wait()
	if s.objectStorage != nil {
		if err := s.objectStorage.Close(); err != nil {
			s.Logger.Warn("datastorage", "failed to close object storage", logging.ErrorField(err))
		}
	}
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Service) CreateEntity(ctx context.Context, entity *model.Entity) error {
	return s.entityStore.CreateEntity(ctx, entity)
}

func (s *Service) GetEntity(ctx context.Context, entityID string) (*model.Entity, error) {
	return s.entityStore.GetEntity(ctx, entityID)
}

func (s *Service) ListEntities(ctx context.Context, filters ...store.EntityFilter) ([]model.Entity, error) {
	return s.entityStore.ListEntities(ctx, filters...)
}

func (s *Service) UpdateEntity(ctx context.Context, entity *model.Entity) error {
	return s.entityStore.UpdateEntity(ctx, entity)
}

func (s *Service) DeleteEntity(ctx context.Context, entityID string) error {
	return s.entityStore.DeleteEntity(ctx, entityID)
}

func (s *Service) UpsertEntity(ctx context.Context, entity *model.Entity) error {
	return s.entityStore.UpsertEntity(ctx, entity)
}

func (s *Service) CreateTask(ctx context.Context, task *model.Task) error {
	return s.taskStore.CreateTask(ctx, task)
}

func (s *Service) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	return s.taskStore.GetTask(ctx, taskID)
}

func (s *Service) ListTasks(ctx context.Context, filters ...store.TaskFilter) ([]model.Task, error) {
	return s.taskStore.ListTasks(ctx, filters...)
}

func (s *Service) UpdateTask(ctx context.Context, task *model.Task) error {
	return s.taskStore.UpdateTask(ctx, task)
}

func (s *Service) DeleteTask(ctx context.Context, taskID string) error {
	return s.taskStore.DeleteTask(ctx, taskID)
}

func (s *Service) UpsertTask(ctx context.Context, task *model.Task) error {
	return s.taskStore.UpsertTask(ctx, task)
}

func (s *Service) CreateObservation(ctx context.Context, observation *model.Observation) error {
	return s.observationStore.CreateObservation(ctx, observation)
}

func (s *Service) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	return s.observationStore.GetObservation(ctx, observationID)
}

func (s *Service) ListObservations(ctx context.Context, filters ...store.ObservationFilter) ([]model.Observation, error) {
	return s.observationStore.ListObservations(ctx, filters...)
}

func (s *Service) UpdateObservation(ctx context.Context, observation *model.Observation) error {
	return s.observationStore.UpdateObservation(ctx, observation)
}

func (s *Service) DeleteObservation(ctx context.Context, observationID string) error {
	return s.observationStore.DeleteObservation(ctx, observationID)
}

func (s *Service) UpsertObservation(ctx context.Context, observation *model.Observation) error {
	return s.observationStore.UpsertObservation(ctx, observation)
}

func (s *Service) ClaimIdempotency(ctx context.Context, scope, key, resourceID string) (store.IdempotencyRecord, bool, error) {
	return s.idempotencyStore.TryBegin(ctx, scope, key, resourceID)
}

func (s *Service) MarkIdempotencyCompleted(ctx context.Context, scope, key string) error {
	return s.idempotencyStore.MarkCompleted(ctx, scope, key)
}

func (s *Service) MarkIdempotencyFailed(ctx context.Context, scope, key string) error {
	return s.idempotencyStore.MarkFailed(ctx, scope, key)
}
