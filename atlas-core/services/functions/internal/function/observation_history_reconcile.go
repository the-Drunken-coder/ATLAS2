package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

// afterHistoryReconcilePolicy controls which row-write errors trigger reconcileAfterHistoryAppend.
type afterHistoryReconcilePolicy struct {
	reconcileOnVersionConflict bool
}

var (
	afterHistoryCRUD   = afterHistoryReconcilePolicy{reconcileOnVersionConflict: false}
	afterHistoryIngest = afterHistoryReconcilePolicy{reconcileOnVersionConflict: true}
)

// reconcileableAfterHistoryError reports whether a failed row write may be recovered via reconcile.
func reconcileableAfterHistoryError(err error, policy afterHistoryReconcilePolicy) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, model.ErrConflict) || errors.Is(err, model.ErrNotFound) {
		return false
	}
	if errors.Is(err, model.ErrVersionConflict) {
		return policy.reconcileOnVersionConflict
	}
	return true
}

// reconcileAfterHistoryAppend re-applies a durable history line to the observation row after
// append succeeded but the row write failed. event_id dedup prevents duplicate history lines.
// allowCreateOnNotFound is true only for create-after-history recovery; update/ingest must not
// recreate a row that was deleted before reconcile reloads.
func (f ObservationFunctions) reconcileAfterHistoryAppend(ctx context.Context, overlay *model.Observation, historyObjectID string, eventLine []byte, allowCreateOnNotFound bool) (*model.Observation, error) {
	var evt observationHistoryEvent
	if err := json.Unmarshal(bytes.TrimSpace(eventLine), &evt); err != nil {
		return nil, err
	}
	if err := f.appendHistoryEventIfAbsent(ctx, historyObjectID, eventLine); err != nil {
		return nil, err
	}
	reloaded, err := f.pgStore.GetObservation(ctx, overlay.ObservationID)
	if errors.Is(err, model.ErrNotFound) {
		if !allowCreateOnNotFound {
			return nil, model.ErrNotFound
		}
		candidate := *overlay
		if err := applyHistoryEventToObservation(&candidate, evt, historyObjectID); err != nil {
			return nil, err
		}
		candidate.UpdatedAt = time.Now().UTC()
		if err := f.pgStore.CreateObservation(ctx, &candidate); err != nil {
			return nil, err
		}
		return &candidate, nil
	}
	if err != nil {
		return nil, err
	}
	applyObservationRowOverlay(reloaded, overlay)
	if err := applyHistoryEventToObservation(reloaded, evt, historyObjectID); err != nil {
		return nil, err
	}
	reloaded.UpdatedAt = time.Now().UTC()
	if err := f.pgStore.UpdateObservation(ctx, reloaded); err != nil {
		return nil, err
	}
	return reloaded, nil
}

func (f ObservationFunctions) updateObservationAfterHistory(ctx context.Context, overlay, storeObs *model.Observation, historyObjectID string, eventLine []byte, policy afterHistoryReconcilePolicy) error {
	if err := f.pgStore.UpdateObservation(ctx, storeObs); err == nil {
		return nil
	} else if len(eventLine) == 0 || !reconcileableAfterHistoryError(err, policy) {
		return err
	}
	reloaded, reconcileErr := f.reconcileAfterHistoryAppend(ctx, overlay, historyObjectID, eventLine, false)
	if reconcileErr != nil {
		return reconcileErr
	}
	*storeObs = *reloaded
	return nil
}

func (f ObservationFunctions) createObservationAfterHistory(ctx context.Context, overlay *model.Observation, historyObjectID string, eventLine []byte, policy afterHistoryReconcilePolicy) error {
	if err := f.pgStore.CreateObservation(ctx, overlay); err == nil {
		return nil
	} else if len(eventLine) == 0 || !reconcileableAfterHistoryError(err, policy) {
		return err
	}
	reloaded, reconcileErr := f.reconcileAfterHistoryAppend(ctx, overlay, historyObjectID, eventLine, true)
	if reconcileErr != nil {
		return reconcileErr
	}
	*overlay = *reloaded
	return nil
}

// applyObservationRowOverlay copies caller-intended scalar fields from overlay onto the reloaded row.
func applyObservationRowOverlay(dst, overlay *model.Observation) {
	if overlay.EndedAt != nil {
		dst.EndedAt = overlay.EndedAt
	}
	if overlay.TargetEntityID != nil {
		dst.TargetEntityID = overlay.TargetEntityID
	}
	if !overlay.StartedAt.IsZero() {
		dst.StartedAt = overlay.StartedAt
	}
	if overlay.SourceAssetID != "" {
		dst.SourceAssetID = overlay.SourceAssetID
	}
}
