package function

import (
	"bytes"
	"strings"

	"atlas.local/protocol"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/objectpath"
)

func requireModel[T any](value *T, field string) error {
	if value == nil {
		return model.NewFieldError("INVALID_INPUT", field+" is required", field)
	}
	return nil
}

func validateEntityModel(entity *model.Entity) error {
	if err := requireModel(entity, "entity"); err != nil {
		return err
	}
	if entity.EntityID == "" {
		return model.NewFieldError("INVALID_INPUT", "entity_id is required", "entity_id")
	}
	if len(entity.EntityID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "entity_id must be 1-50 characters", "entity_id")
	}
	if entity.Type != model.EntityTypeAsset && entity.Type != model.EntityTypeTrack && entity.Type != model.EntityTypeGeofeature {
		return model.NewFieldError("INVALID_INPUT", "type must be asset, track, or geofeature", "type")
	}
	return nil
}

func validateObjectModel(obj *model.Object) error {
	if err := requireModel(obj, "object"); err != nil {
		return err
	}
	if obj.ObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "object_id is required", "object_id")
	}
	if len(obj.ObjectID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "object_id must be 1-50 characters", "object_id")
	}
	if obj.Type == "" {
		return model.NewFieldError("INVALID_INPUT", "type is required", "type")
	}
	if !isKnownObjectType(obj.Type) {
		return model.NewFieldError("INVALID_INPUT", "type must be one of: "+knownObjectTypesCSV(), "type")
	}
	if obj.OwnerType != model.OwnerTypeEntity && obj.OwnerType != model.OwnerTypeObservation && obj.OwnerType != model.OwnerTypeTask && obj.OwnerType != model.OwnerTypeSystem {
		return model.NewFieldError("INVALID_INPUT", "owner_type must be entity, observation, task, or system", "owner_type")
	}
	if obj.OwnerID == "" {
		return model.NewFieldError("INVALID_INPUT", "owner_id is required", "owner_id")
	}
	if err := objectpath.ValidateObjectID(obj.ObjectID); err != nil {
		return model.NewFieldError("INVALID_INPUT", err.Error(), "object_id")
	}
	return nil
}

func validateTaskModel(task *model.Task) error {
	if err := requireModel(task, "task"); err != nil {
		return err
	}
	if task.TaskID == "" {
		return model.NewFieldError("INVALID_INPUT", "task_id is required", "task_id")
	}
	if len(task.TaskID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "task_id must be 1-50 characters", "task_id")
	}
	if task.Status != model.TaskStatusPending && task.Status != model.TaskStatusAcknowledged && task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusFailed {
		return model.NewFieldError("INVALID_INPUT", "status must be pending, acknowledged, completed, or failed", "status")
	}
	if task.AssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "asset_id is required", "asset_id")
	}
	if task.CommandCatalogObjectID == "" {
		return model.NewFieldError("INVALID_INPUT", "command_catalog_object_id is required", "command_catalog_object_id")
	}
	return nil
}

// minimumObservationJSON satisfies protocol minProperties without carrying domain data.
var minimumObservationJSON = []byte(`{"extra":{}}`)

func validateObservationJSON(json []byte) error {
	if json == nil {
		return model.NewFieldError("INVALID_INPUT", "json is required", "json")
	}
	trimmed := bytes.TrimSpace(json)
	if len(trimmed) == 0 {
		return model.NewFieldError("INVALID_INPUT", "json is required", "json")
	}
	if len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' {
		if len(bytes.TrimSpace(trimmed[1:len(trimmed)-1])) == 0 {
			return model.NewFieldError("INVALID_INPUT", "observation json must include at least one property", "json")
		}
	}
	return nil
}

func validateObservationModel(obs *model.Observation, requireStartedAt bool) error {
	if err := requireModel(obs, "observation"); err != nil {
		return err
	}
	if obs.ObservationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	if len(obs.ObservationID) > 50 {
		return model.NewFieldError("INVALID_INPUT", "observation_id must be 1-50 characters", "observation_id")
	}
	if obs.SourceAssetID == "" {
		return model.NewFieldError("INVALID_INPUT", "source_asset_id is required", "source_asset_id")
	}
	if requireStartedAt && obs.StartedAt.IsZero() {
		return model.NewFieldError("INVALID_INPUT", "started_at is required", "started_at")
	}
	return nil
}

func isKnownObjectType(objectType model.ObjectType) bool {
	for _, known := range model.KnownObjectTypes() {
		if known == objectType {
			return true
		}
	}
	return false
}

func knownObjectTypesCSV() string {
	known := model.KnownObjectTypes()
	out := make([]string, 0, len(known))
	for _, t := range known {
		out = append(out, string(t))
	}
	return strings.Join(out, ", ")
}

type noopProtocolValidator struct{}

func (noopProtocolValidator) ValidateEntity(*model.Entity) []protocol.ValidationIssue { return nil }
func (noopProtocolValidator) ValidateObject(*model.Object) []protocol.ValidationIssue { return nil }
func (noopProtocolValidator) ValidateTask(*model.Task) []protocol.ValidationIssue     { return nil }
func (noopProtocolValidator) ValidateObservation(*model.Observation) []protocol.ValidationIssue {
	return nil
}
func (noopProtocolValidator) ValidateObservationHistoryEvent([]byte) []protocol.ValidationIssue {
	return nil
}
func (noopProtocolValidator) ValidateCommandCatalogJSON([]byte) []protocol.ValidationIssue {
	return nil
}

