package blob

import "github.com/anomalyco/atlas-core/internal/core/model"

type Operation string

const (
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationUpsert Operation = "upsert"
)

func NormalizeEntity(entity *model.Entity, op Operation) error {
	root, violations, err := parseAndNormalize(entity.JSON)
	if err != nil {
		return err
	}
	validateEntity(root, entity.Type, op, &violations)
	if err := newValidationError(violations); err != nil {
		return err
	}
	entity.JSON, err = canonicalJSON(root)
	return err
}

func NormalizeObject(obj *model.Object, op Operation) error {
	root, violations, err := parseAndNormalize(obj.JSON)
	if err != nil {
		return err
	}
	validateObject(root, obj.Type, obj.ObjectID, op, &violations)
	if err := newValidationError(violations); err != nil {
		return err
	}
	obj.JSON, err = canonicalJSON(root)
	return err
}

func NormalizeTask(task *model.Task, op Operation) error {
	root, violations, err := parseAndNormalize(task.JSON)
	if err != nil {
		return err
	}
	validateTask(root, op, &violations)
	if err := newValidationError(violations); err != nil {
		return err
	}
	task.JSON, err = canonicalJSON(root)
	return err
}

func NormalizeObservation(obs *model.Observation, op Operation) error {
	root, violations, err := parseAndNormalize(obs.JSON)
	if err != nil {
		return err
	}
	validateObservation(root, op, &violations)
	if err := newValidationError(violations); err != nil {
		return err
	}
	obs.JSON, err = canonicalJSON(root)
	return err
}
