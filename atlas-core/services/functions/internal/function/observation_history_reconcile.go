package function

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func (f ObservationFunctions) reconcileAfterHistoryAppend(ctx context.Context, obs *model.Observation, historyObjectID string, eventLine []byte) error {
	var evt observationHistoryEvent
	if err := json.Unmarshal(bytes.TrimSpace(eventLine), &evt); err != nil {
		return err
	}
	exists, err := f.historyContainsEventID(ctx, historyObjectID, evt.EventID)
	if err != nil {
		return err
	}
	if !exists {
		if err := f.appendHistoryEvent(ctx, historyObjectID, eventLine); err != nil {
			return err
		}
	}
	reloaded, err := f.pgStore.GetObservation(ctx, obs.ObservationID)
	if err != nil {
		return err
	}
	if err := applyHistoryEventToObservation(reloaded, evt, historyObjectID); err != nil {
		return err
	}
	reloaded.UpdatedAt = time.Now().UTC()
	return f.pgStore.UpdateObservation(ctx, reloaded)
}
