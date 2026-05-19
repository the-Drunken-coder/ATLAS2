package service

import (
	"context"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	functionsv1.UnimplementedAtlasFunctionsServiceServer
	functionsv1.UnimplementedChangefeedServiceServer
	funcs functionpkg.Functions
	hub   *changefeed.Hub
	log   *logging.Logger
	// testPublishObjectUpdated is set only in tests to simulate publish failures.
	testPublishObjectUpdated func(context.Context, string) error
}

func NewServer(funcs functionpkg.Functions, hub *changefeed.Hub, log *logging.Logger) *Server {
	if hub == nil {
		hub = changefeed.NewHub()
	}
	return &Server{funcs: funcs, hub: hub, log: log}
}

func (s *Server) status(ctx context.Context, err error) error {
	return rpcerrors.ToStatusContext(ctx, s.log, err)
}

func (s *Server) publishObjectUpdated(ctx context.Context, objectID string) error {
	if s.testPublishObjectUpdated != nil {
		return s.testPublishObjectUpdated(ctx, objectID)
	}
	return s.funcs.Object.PublishObjectUpdated(ctx, objectID)
}

func RegisterGRPC(server grpc.ServiceRegistrar, funcs functionpkg.Functions, hub *changefeed.Hub, log *logging.Logger) {
	handler := NewServer(funcs, hub, log)
	functionsv1.RegisterAtlasFunctionsServiceServer(server, handler)
	functionsv1.RegisterChangefeedServiceServer(server, handler)
}

func defaultEntityRequestTimestamps(entity *sharedv1.Entity) *sharedv1.Entity {
	if entity == nil || (entity.GetCreatedAt() != nil && entity.GetUpdatedAt() != nil) {
		return entity
	}
	copy := *entity
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultObjectRequestTimestamps(object *sharedv1.Object) *sharedv1.Object {
	if object == nil || (object.GetCreatedAt() != nil && object.GetUpdatedAt() != nil) {
		return object
	}
	copy := *object
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultTaskRequestTimestamps(task *sharedv1.Task) *sharedv1.Task {
	if task == nil || (task.GetCreatedAt() != nil && task.GetUpdatedAt() != nil) {
		return task
	}
	copy := *task
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}

func defaultObservationRequestTimestamps(observation *sharedv1.Observation) *sharedv1.Observation {
	if observation == nil || (observation.GetCreatedAt() != nil && observation.GetUpdatedAt() != nil) {
		return observation
	}
	copy := *observation
	now := timestamppb.Now()
	if copy.CreatedAt == nil {
		copy.CreatedAt = now
	}
	if copy.UpdatedAt == nil {
		copy.UpdatedAt = now
	}
	return &copy
}
