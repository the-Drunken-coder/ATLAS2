package function

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func (f ObservationFunctions) reconcileAfterHistoryAppend(ctx context.Context, overlay *model.Observation, historyObjectID string, eventLine []byte) error {
	var evt observationHistoryEvent
	if err := json.Unmarshal(bytes.TrimSpace(eventLine), &evt); err != nil {
		return err
	}
	if err := f.appendHistoryEventIfAbsent(ctx, historyObjectID, eventLine); err != nil {
		return err
	}
	reloaded, err := f.pgStore.GetObservation(ctx, overlay.ObservationID)
	if err != nil {
		return err
	}
	applyIngestLifecycleOverlay(reloaded, overlay)
	if err := applyHistoryEventToObservation(reloaded, evt, historyObjectID); err != nil {
		return err
	}
	reloaded.UpdatedAt = time.Now().UTC()
	return f.pgStore.UpdateObservation(ctx, reloaded)
}

// applyIngestLifecycleOverlay copies ingest-scalar fields from overlay onto the reloaded row.
func applyIngestLifecycleOverlay(dst, overlay *model.Observation) {
	if overlay.EndedAt != nil {
		dst.EndedAt = overlay.EndedAt
	}
	if overlay.TargetEntityID != nil {
		dst.TargetEntityID = overlay.TargetEntityID
	}
}
