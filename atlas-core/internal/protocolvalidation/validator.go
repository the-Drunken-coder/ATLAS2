package protocolvalidation

import (
	"atlas.local/protocol"

	"github.com/anomalyco/atlas-core/internal/model"
)

type Validator struct {
	v *protocol.Validator
}

func New() (*Validator, error) {
	v, err := protocol.New()
	if err != nil {
		return nil, err
	}
	return &Validator{v: v}, nil
}

func (v *Validator) ValidateEntity(entity *model.Entity) []protocol.ValidationIssue {
	return v.v.ValidateBytes(protocol.ResourceEntity, entity.JSON, protocol.WithVariant(string(entity.Type)))
}

func (v *Validator) ValidateObject(obj *model.Object) []protocol.ValidationIssue {
	if obj.Type == model.ObjectTypeCommandCatalog {
		return v.v.ValidateBytes(protocol.ResourceCommandCatalog, obj.JSON)
	}
	variant := string(obj.Type)
	if variant == "" {
		return []protocol.ValidationIssue{{Field: "json", Code: "invalid_value", Message: "object variant is required"}}
	}
	return v.v.ValidateBytes(protocol.ResourceObject, obj.JSON, protocol.WithVariant(variant))
}

func (v *Validator) ValidateTask(task *model.Task) []protocol.ValidationIssue {
	return v.v.ValidateBytes(protocol.ResourceTask, task.JSON)
}

func (v *Validator) ValidateObservation(obs *model.Observation) []protocol.ValidationIssue {
	return v.v.ValidateBytes(protocol.ResourceObservation, obs.JSON)
}

func (v *Validator) ValidateCommandCatalogJSON(json []byte) []protocol.ValidationIssue {
	return v.v.ValidateBytes(protocol.ResourceCommandCatalog, json)
}
