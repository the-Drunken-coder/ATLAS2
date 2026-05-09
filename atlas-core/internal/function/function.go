package function

import (
	"github.com/anomalyco/atlas-core/internal/logging"
	"github.com/anomalyco/atlas-core/internal/store"
)

type Functions struct {
	Entity      EntityFunctions
	Object      ObjectFunctions
	Task        TaskFunctions
	Observation ObservationFunctions
}

func NewEntityFunctions(pgStore store.EntityStore, log *logging.Logger) EntityFunctions {
	return EntityFunctions{pgStore: pgStore, log: log}
}

func NewObjectFunctions(pgStore store.ObjectStore, objStore store.ObjectStorageStore, idemStore store.IdempotencyStore, log *logging.Logger) ObjectFunctions {
	return ObjectFunctions{pgStore: pgStore, objStore: objStore, idemStore: idemStore, log: log}
}

func NewTaskFunctions(taskStore store.TaskStore, entityStore store.EntityStore, objectStore store.ObjectStore, idemStore store.IdempotencyStore, log *logging.Logger) TaskFunctions {
	return TaskFunctions{taskStore: taskStore, entityStore: entityStore, objectStore: objectStore, idemStore: idemStore, log: log}
}

func NewObservationFunctions(pgStore store.ObservationStore, log *logging.Logger) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log}
}
