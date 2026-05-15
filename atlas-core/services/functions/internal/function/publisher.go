package function

import (
	"context"
	"fmt"
	"time"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Publisher interface {
	Publish(context.Context, *sharedv1.MutationEvent)
}

type NopPublisher struct{}

func (NopPublisher) Publish(context.Context, *sharedv1.MutationEvent) {}

func publisherOrNop(publishers []Publisher) Publisher {
	if len(publishers) > 0 && publishers[0] != nil {
		return publishers[0]
	}
	return NopPublisher{}
}

func publishEntity(ctx context.Context, publisher Publisher, operation string, entity *model.Entity) {
	if entity == nil {
		return
	}
	event := baseMutationEvent("entity", operation, entity.EntityID, entity.Version)
	if operation != "deleted" {
		event.Snapshot = &sharedv1.MutationEvent_Entity{Entity: pbconv.EntityToProto(entity)}
	}
	publisher.Publish(ctx, event)
}

func publishObject(ctx context.Context, publisher Publisher, operation string, object *model.Object) {
	if object == nil {
		return
	}
	event := baseMutationEvent("object", operation, object.ObjectID, object.Version)
	if operation != "deleted" {
		event.Snapshot = &sharedv1.MutationEvent_Object{Object: pbconv.ObjectToProto(object)}
	}
	publisher.Publish(ctx, event)
}

func publishObjectID(ctx context.Context, publisher Publisher, operation, objectID string) {
	if objectID == "" {
		return
	}
	event := baseMutationEvent("object", operation, objectID, 0)
	publisher.Publish(ctx, event)
}

func publishTask(ctx context.Context, publisher Publisher, operation string, task *model.Task) {
	if task == nil {
		return
	}
	event := baseMutationEvent("task", operation, task.TaskID, task.Version)
	if operation != "deleted" {
		event.Snapshot = &sharedv1.MutationEvent_Task{Task: pbconv.TaskToProto(task)}
	}
	publisher.Publish(ctx, event)
}

func publishObservation(ctx context.Context, publisher Publisher, operation string, observation *model.Observation) {
	if observation == nil {
		return
	}
	event := baseMutationEvent("observation", operation, observation.ObservationID, observation.Version)
	if operation != "deleted" {
		event.Snapshot = &sharedv1.MutationEvent_Observation{Observation: pbconv.ObservationToProto(observation)}
	}
	publisher.Publish(ctx, event)
}

func baseMutationEvent(resource, operation, resourceID string, version int) *sharedv1.MutationEvent {
	occurredAt := time.Now().UTC()
	return &sharedv1.MutationEvent{
		EventId:         fmt.Sprintf("evt-%s-%s-%s-v%d-%d", resource, resourceID, operation, version, occurredAt.UnixNano()),
		Resource:        resource,
		Operation:       operation,
		ResourceId:      resourceID,
		ResourceVersion: int32(version),
		OccurredAt:      timestamppb.New(occurredAt),
		Metadata:        map[string]string{"source": "atlas-functions"},
	}
}
