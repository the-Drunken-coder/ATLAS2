package service

import (
	"github.com/anomalyco/atlas-core/internal/core/ports"
	"github.com/anomalyco/atlas-core/internal/protocolvalidation"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

type Functions struct {
	Entity      EntityFunctions
	Object      ObjectFunctions
	Task        TaskFunctions
	Observation ObservationFunctions
}

func NewEntityFunctions(pgStore ports.EntityStore, log *logging.Logger) EntityFunctions {
	return EntityFunctions{pgStore: pgStore, log: log, validator: protocolvalidation.NewRunner()}
}

func NewObjectFunctions(pgStore ports.ObjectStore, objStore ports.ObjectStorageStore, idemStore ports.IdempotencyStore, log *logging.Logger) ObjectFunctions {
	return ObjectFunctions{pgStore: pgStore, objStore: objStore, idemStore: idemStore, log: log, validator: protocolvalidation.NewRunner()}
}

func NewTaskFunctions(taskStore ports.TaskStore, entityStore ports.EntityStore, objectStore ports.ObjectStore, idemStore ports.IdempotencyStore, log *logging.Logger) TaskFunctions {
	return TaskFunctions{taskStore: taskStore, entityStore: entityStore, objectStore: objectStore, idemStore: idemStore, log: log, validator: protocolvalidation.NewRunner()}
}

func NewObservationFunctions(pgStore ports.ObservationStore, log *logging.Logger) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log, validator: protocolvalidation.NewRunner()}
}
