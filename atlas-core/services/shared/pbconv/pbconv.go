package pbconv

import (
	"errors"
	"fmt"
	"time"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	errTimestampRequired = errors.New("timestamp is required")
	errTimestampInvalid  = errors.New("timestamp is invalid")
)

func EntityToProto(entity *model.Entity) *sharedv1.Entity {
	if entity == nil {
		return nil
	}
	out := &sharedv1.Entity{
		EntityId:  entity.EntityID,
		Type:      string(entity.Type),
		Json:      append([]byte(nil), entity.JSON...),
		Version:   int32(entity.Version),
		CreatedAt: timestamppb.New(entity.CreatedAt.UTC()),
		UpdatedAt: timestamppb.New(entity.UpdatedAt.UTC()),
	}
	if entity.Subtype != nil {
		out.Subtype = stringPtr(*entity.Subtype)
	}
	if entity.Alias != nil {
		out.Alias = stringPtr(*entity.Alias)
	}
	return out
}

func EntityFromProto(entity *sharedv1.Entity) (*model.Entity, error) {
	if entity == nil {
		return nil, fmt.Errorf("entity is required")
	}
	createdAt, err := timestampValue(entity.GetCreatedAt(), "entity.created_at")
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampValue(entity.GetUpdatedAt(), "entity.updated_at")
	if err != nil {
		return nil, err
	}
	out := &model.Entity{
		EntityID:  entity.GetEntityId(),
		Type:      model.EntityType(entity.GetType()),
		JSON:      append([]byte(nil), entity.GetJson()...),
		Version:   int(entity.GetVersion()),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if entity.Subtype != nil {
		out.Subtype = stringPtr(*entity.Subtype)
	}
	if entity.Alias != nil {
		out.Alias = stringPtr(*entity.Alias)
	}
	return out, nil
}

func ObjectToProto(object *model.Object) *sharedv1.Object {
	if object == nil {
		return nil
	}
	return &sharedv1.Object{
		ObjectId:  object.ObjectID,
		Type:      string(object.Type),
		OwnerType: string(object.OwnerType),
		OwnerId:   object.OwnerID,
		Json:      append([]byte(nil), object.JSON...),
		Version:   int32(object.Version),
		CreatedAt: timestamppb.New(object.CreatedAt.UTC()),
		UpdatedAt: timestamppb.New(object.UpdatedAt.UTC()),
	}
}

func ObjectFromProto(object *sharedv1.Object) (*model.Object, error) {
	if object == nil {
		return nil, fmt.Errorf("object is required")
	}
	createdAt, err := timestampValue(object.GetCreatedAt(), "object.created_at")
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampValue(object.GetUpdatedAt(), "object.updated_at")
	if err != nil {
		return nil, err
	}
	return &model.Object{
		ObjectID:  object.GetObjectId(),
		Type:      model.ObjectType(object.GetType()),
		OwnerType: model.OwnerType(object.GetOwnerType()),
		OwnerID:   object.GetOwnerId(),
		JSON:      append([]byte(nil), object.GetJson()...),
		Version:   int(object.GetVersion()),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func TaskToProto(task *model.Task) *sharedv1.Task {
	if task == nil {
		return nil
	}
	return &sharedv1.Task{
		TaskId:                 task.TaskID,
		Status:                 string(task.Status),
		AssetId:                task.AssetID,
		CommandCatalogObjectId: task.CommandCatalogObjectID,
		Json:                   append([]byte(nil), task.JSON...),
		Version:                int32(task.Version),
		CreatedAt:              timestamppb.New(task.CreatedAt.UTC()),
		UpdatedAt:              timestamppb.New(task.UpdatedAt.UTC()),
	}
}

func TaskFromProto(task *sharedv1.Task) (*model.Task, error) {
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	createdAt, err := timestampValue(task.GetCreatedAt(), "task.created_at")
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampValue(task.GetUpdatedAt(), "task.updated_at")
	if err != nil {
		return nil, err
	}
	return &model.Task{
		TaskID:                 task.GetTaskId(),
		Status:                 model.TaskStatus(task.GetStatus()),
		AssetID:                task.GetAssetId(),
		CommandCatalogObjectID: task.GetCommandCatalogObjectId(),
		JSON:                   append([]byte(nil), task.GetJson()...),
		Version:                int(task.GetVersion()),
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func ObservationToProto(observation *model.Observation) *sharedv1.Observation {
	if observation == nil {
		return nil
	}
	out := &sharedv1.Observation{
		ObservationId: observation.ObservationID,
		SourceAssetId: observation.SourceAssetID,
		Json:          append([]byte(nil), observation.JSON...),
		Version:       int32(observation.Version),
		CreatedAt:     timestamppb.New(observation.CreatedAt.UTC()),
		UpdatedAt:     timestamppb.New(observation.UpdatedAt.UTC()),
	}
	if observation.TargetEntityID != nil {
		out.TargetEntityId = stringPtr(*observation.TargetEntityID)
	}
	if observation.ObservedAt != nil {
		out.ObservedAt = timestamppb.New(observation.ObservedAt.UTC())
	}
	return out
}

func ObservationFromProto(observation *sharedv1.Observation) (*model.Observation, error) {
	if observation == nil {
		return nil, fmt.Errorf("observation is required")
	}
	createdAt, err := timestampValue(observation.GetCreatedAt(), "observation.created_at")
	if err != nil {
		return nil, err
	}
	updatedAt, err := timestampValue(observation.GetUpdatedAt(), "observation.updated_at")
	if err != nil {
		return nil, err
	}
	out := &model.Observation{
		ObservationID: observation.GetObservationId(),
		SourceAssetID: observation.GetSourceAssetId(),
		JSON:          append([]byte(nil), observation.GetJson()...),
		Version:       int(observation.GetVersion()),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if observation.TargetEntityId != nil {
		out.TargetEntityID = stringPtr(observation.GetTargetEntityId())
	}
	if observation.ObservedAt != nil {
		observedAt, err := optionalTimestampValue(observation.GetObservedAt(), "observation.observed_at")
		if err != nil {
			return nil, err
		}
		utc := observedAt.UTC()
		out.ObservedAt = &utc
	}
	return out, nil
}

func ManifestToProto(manifest *model.ObjectManifest) *sharedv1.ObjectManifest {
	if manifest == nil {
		return nil
	}
	out := &sharedv1.ObjectManifest{Version: manifest.Version, Files: map[string]*sharedv1.ObjectFileInfo{}}
	for name, info := range manifest.Files {
		out.Files[name] = &sharedv1.ObjectFileInfo{Size: info.Size, UpdatedAt: timestamppb.New(info.UpdatedAt.UTC())}
	}
	return out
}

func ManifestFromProto(manifest *sharedv1.ObjectManifest) (*model.ObjectManifest, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is required")
	}
	out := &model.ObjectManifest{Version: manifest.GetVersion(), Files: map[string]model.ObjectFileInfo{}}
	for name, info := range manifest.GetFiles() {
		updatedAt, err := timestampValue(info.GetUpdatedAt(), fmt.Sprintf("manifest.files[%q].updated_at", name))
		if err != nil {
			return nil, err
		}
		out.Files[name] = model.ObjectFileInfo{Size: info.GetSize(), UpdatedAt: updatedAt}
	}
	return model.NormalizeManifest(out), nil
}

func EntityFiltersFromProto(filter *sharedv1.EntityFilter) ([]store.EntityFilter, error) {
	if filter == nil {
		return nil, nil
	}
	var out []store.EntityFilter
	if filter.Type != nil {
		out = append(out, store.WithEntityType(model.EntityType(filter.GetType())))
	}
	if filter.UpdatedAfter != nil {
		updatedAfter, err := optionalTimestampValue(filter.GetUpdatedAfter(), "updated_after")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithEntityUpdatedAfter(updatedAfter))
	}
	return out, nil
}

func ObjectFiltersFromProto(filter *sharedv1.ObjectFilter) ([]store.ObjectFilter, error) {
	if filter == nil {
		return nil, nil
	}
	var out []store.ObjectFilter
	if filter.OwnerType != nil && filter.OwnerId != nil {
		out = append(out, store.WithObjectOwner(model.OwnerType(filter.GetOwnerType()), filter.GetOwnerId()))
	} else {
		if filter.OwnerType != nil {
			out = append(out, store.WithObjectOwnerType(model.OwnerType(filter.GetOwnerType())))
		}
		if filter.OwnerId != nil {
			out = append(out, store.WithObjectOwnerID(filter.GetOwnerId()))
		}
	}
	if filter.ObjectType != nil {
		out = append(out, store.WithObjectType(model.ObjectType(filter.GetObjectType())))
	}
	if filter.UpdatedAfter != nil {
		updatedAfter, err := optionalTimestampValue(filter.GetUpdatedAfter(), "updated_after")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithObjectUpdatedAfter(updatedAfter))
	}
	return out, nil
}

func TaskFiltersFromProto(filter *sharedv1.TaskFilter) ([]store.TaskFilter, error) {
	if filter == nil {
		return nil, nil
	}
	var out []store.TaskFilter
	if filter.AssetId != nil {
		out = append(out, store.WithTaskAssetID(filter.GetAssetId()))
	}
	if filter.Status != nil {
		out = append(out, store.WithTaskStatus(model.TaskStatus(filter.GetStatus())))
	}
	if filter.UpdatedAfter != nil {
		updatedAfter, err := optionalTimestampValue(filter.GetUpdatedAfter(), "updated_after")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithTaskUpdatedAfter(updatedAfter))
	}
	return out, nil
}

func ObservationFiltersFromProto(filter *sharedv1.ObservationFilter) ([]store.ObservationFilter, error) {
	if filter == nil {
		return nil, nil
	}
	var out []store.ObservationFilter
	if filter.SourceAssetId != nil {
		out = append(out, store.WithObservationSourceAssetID(filter.GetSourceAssetId()))
	}
	if filter.TargetEntityId != nil {
		out = append(out, store.WithObservationTargetEntityID(filter.GetTargetEntityId()))
	}
	if filter.ObservedAtFrom != nil {
		observedAtFrom, err := optionalTimestampValue(filter.GetObservedAtFrom(), "observed_at_from")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithObservationObservedAtFrom(observedAtFrom))
	}
	if filter.ObservedAtTo != nil {
		observedAtTo, err := optionalTimestampValue(filter.GetObservedAtTo(), "observed_at_to")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithObservationObservedAtTo(observedAtTo))
	}
	if filter.UpdatedAfter != nil {
		updatedAfter, err := optionalTimestampValue(filter.GetUpdatedAfter(), "updated_after")
		if err != nil {
			return nil, err
		}
		out = append(out, store.WithObservationUpdatedAfter(updatedAfter))
	}
	return out, nil
}

func EntityFilterToProto(filters []store.EntityFilter) *sharedv1.EntityFilter {
	state := &store.EntityFilterState{}
	for _, filter := range filters {
		filter(state)
	}
	out := &sharedv1.EntityFilter{}
	if state.EntityType != nil {
		out.Type = stringPtr(string(*state.EntityType))
	}
	if state.UpdatedAfter != nil {
		out.UpdatedAfter = timestamppb.New(state.UpdatedAfter.UTC())
	}
	return out
}

func ObjectFilterToProto(filters []store.ObjectFilter) *sharedv1.ObjectFilter {
	state := &store.ObjectFilterState{}
	for _, filter := range filters {
		filter(state)
	}
	out := &sharedv1.ObjectFilter{}
	if state.OwnerType != nil {
		out.OwnerType = stringPtr(string(*state.OwnerType))
	}
	if state.OwnerID != nil {
		out.OwnerId = stringPtr(*state.OwnerID)
	}
	if state.ObjectType != nil {
		out.ObjectType = stringPtr(string(*state.ObjectType))
	}
	if state.UpdatedAfter != nil {
		out.UpdatedAfter = timestamppb.New(state.UpdatedAfter.UTC())
	}
	return out
}

func TaskFilterToProto(filters []store.TaskFilter) *sharedv1.TaskFilter {
	state := &store.TaskFilterState{}
	for _, filter := range filters {
		filter(state)
	}
	out := &sharedv1.TaskFilter{}
	if state.AssetID != nil {
		out.AssetId = stringPtr(*state.AssetID)
	}
	if state.Status != nil {
		out.Status = stringPtr(string(*state.Status))
	}
	if state.UpdatedAfter != nil {
		out.UpdatedAfter = timestamppb.New(state.UpdatedAfter.UTC())
	}
	return out
}

func ObservationFilterToProto(filters []store.ObservationFilter) *sharedv1.ObservationFilter {
	state := &store.ObservationFilterState{}
	for _, filter := range filters {
		filter(state)
	}
	out := &sharedv1.ObservationFilter{}
	if state.SourceAssetID != nil {
		out.SourceAssetId = stringPtr(*state.SourceAssetID)
	}
	if state.TargetEntityID != nil {
		out.TargetEntityId = stringPtr(*state.TargetEntityID)
	}
	if state.ObservedAtFrom != nil {
		out.ObservedAtFrom = timestamppb.New(state.ObservedAtFrom.UTC())
	}
	if state.ObservedAtTo != nil {
		out.ObservedAtTo = timestamppb.New(state.ObservedAtTo.UTC())
	}
	if state.UpdatedAfter != nil {
		out.UpdatedAfter = timestamppb.New(state.UpdatedAfter.UTC())
	}
	return out
}

func timestampValue(ts *timestamppb.Timestamp, field string) (time.Time, error) {
	if ts == nil {
		return time.Time{}, fmt.Errorf("%w: %s", errTimestampRequired, field)
	}
	if err := ts.CheckValid(); err != nil {
		return time.Time{}, fmt.Errorf("%w: %s: %v", errTimestampInvalid, field, err)
	}
	return ts.AsTime().UTC(), nil
}

func optionalTimestampValue(ts *timestamppb.Timestamp, field string) (time.Time, error) {
	if err := ts.CheckValid(); err != nil {
		return time.Time{}, model.NewFieldError("INVALID_INPUT", fmt.Sprintf("%s is invalid: %v", field, err), field)
	}
	return ts.AsTime().UTC(), nil
}

func stringPtr(value string) *string {
	return &value
}
