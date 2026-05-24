package function

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

func canonicalizeTelemetryJSON(telemetryJSON []byte) (telemetryEnvelope, error) {
	var telemetry telemetryEnvelope
	if err := json.Unmarshal(telemetryJSON, &telemetry); err != nil {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry must be valid JSON", "telemetry")
	}
	if telemetry.Kind == "" {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry kind is required", "telemetry.kind")
	}
	data := bytes.TrimSpace(telemetry.Data)
	if len(data) == 0 || data[0] != '{' {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be an object", "telemetry.data")
	}
	if !json.Valid(data) {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be valid JSON", "telemetry.data")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return telemetryEnvelope{}, model.NewFieldError("INVALID_INPUT", "telemetry data must be an object", "telemetry.data")
	}
	telemetry.Data = json.RawMessage(data)
	return telemetry, nil
}

func endedAtOrderingValid(startedAt time.Time, endedAt *time.Time) error {
	if endedAt == nil {
		return nil
	}
	if endedAt.Before(startedAt) {
		return model.NewFieldError("INVALID_INPUT", "ended_at must be greater than or equal to started_at", "ended_at")
	}
	return nil
}

func parseIdentityBytes(identityJSON []byte) (json.RawMessage, error) {
	if len(identityJSON) == 0 {
		return nil, nil
	}
	return validateJSONObjectRaw(identityJSON, "identity")
}

type ingestTelemetryPlan struct {
	obs             *model.Observation
	creating        bool
	historyObjectID string
	eventLine       []byte
	telemetry       telemetryEnvelope
	observedAt      time.Time
	identityJSON    []byte
}

// executeIngestTelemetry appends deduped history first, then persists the observation row.
func (f ObservationFunctions) executeIngestTelemetry(ctx context.Context, plan ingestTelemetryPlan) (*model.Observation, error) {
	obs := plan.obs
	if plan.creating {
		if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
			return nil, protocolvalidation.NewValidationError(issues)
		}
		if identity, hasIdentity, err := parseObservationIdentity(obs.JSON); err != nil {
			return nil, err
		} else if hasIdentity {
			if _, err := f.appendIdentityPatchIfNeeded(ctx, obs, nil, identity, obs.StartedAt); err != nil {
				return nil, err
			}
		}
		if err := f.appendTelemetryHistoryIfNeeded(ctx, plan.historyObjectID, plan.eventLine); err != nil {
			return nil, err
		}
		if err := f.createObservationAfterHistory(ctx, obs, plan.historyObjectID, plan.eventLine, afterHistoryIngest); err != nil {
			return nil, err
		}
		publishObservation(ctx, f.publisher, "created", obs)
		return obs, nil
	}

	if err := f.appendTelemetryHistoryIfNeeded(ctx, plan.historyObjectID, plan.eventLine); err != nil {
		return nil, err
	}
	if err := f.updateObservationAfterHistory(ctx, obs, obs, plan.historyObjectID, plan.eventLine, afterHistoryIngest); err != nil {
		return nil, err
	}
	reloaded, err := f.pgStore.GetObservation(ctx, obs.ObservationID)
	if err != nil {
		return nil, err
	}
	obs = reloaded

	if identity, err := parseIdentityBytes(plan.identityJSON); err != nil {
		return nil, err
	} else if len(identity) > 0 {
		previous, hadPrevious, err := parseObservationIdentity(obs.JSON)
		if err != nil {
			return nil, err
		}
		var prev json.RawMessage
		if hadPrevious {
			prev = previous
		}
		if !hadPrevious || identityChanged(prev, identity) {
			commit, err := f.appendIdentityPatchIfNeeded(ctx, obs, prev, identity, plan.observedAt)
			if err != nil {
				return nil, err
			}
			obs.UpdatedAt = time.Now().UTC()
			if err := f.updateObservationAfterHistory(ctx, obs, obs, commit.historyObjectID, commit.eventLine, afterHistoryIngest); err != nil {
				return nil, err
			}
			reloaded, err = f.pgStore.GetObservation(ctx, obs.ObservationID)
			if err != nil {
				return nil, err
			}
			obs = reloaded
		}
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return obs, nil
}
