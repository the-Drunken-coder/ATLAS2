package function

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/functions/internal/gateway"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"github.com/anomalyco/atlas-core/services/shared/store"
)

type ObservationFunctions struct {
	pgStore        store.ObservationStore
	entityStore    store.EntityStore
	objectGateway  gateway.ObjectGateway
	log            *logging.Logger
	protoValidator ProtocolValidator
	publisher      Publisher
}

func NewObservationFunctions(pgStore store.ObservationStore, log *logging.Logger, protoValidator ProtocolValidator, publishers ...Publisher) ObservationFunctions {
	return ObservationFunctions{pgStore: pgStore, log: log, protoValidator: protoValidator, publisher: publisherOrNop(publishers)}
}

func (f ObservationFunctions) WithObjectGateway(gw gateway.ObjectGateway) ObservationFunctions {
	f.objectGateway = gw
	return f
}

func (f ObservationFunctions) WithEntityStore(entityStore store.EntityStore) ObservationFunctions {
	f.entityStore = entityStore
	return f
}

type ObservationTelemetryIngest struct {
	ObservationID  string
	SourceAssetID  string
	TargetEntityID *string
	TelemetryJSON  []byte
	StartedAt      time.Time
	EndedAt        *time.Time
	IdentityJSON   []byte
}

func (f ObservationFunctions) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs, true); err != nil {
		return err
	}
	if err := validateObservationJSON(obs.JSON); err != nil {
		return err
	}
	if observationJSONHasKey(obs.JSON, "latest_telemetry") {
		return model.NewFieldError("INVALID_INPUT", "latest_telemetry is not allowed on create", "json.latest_telemetry")
	}
	if err := rejectClientHistoryObjectID(obs.JSON); err != nil {
		return err
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if err := endedAtOrderingValid(obs.StartedAt, obs.EndedAt); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	if obs.UpdatedAt.IsZero() {
		obs.UpdatedAt = now
	}
	f.log.InfoContext(ctx, "observation", "creating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.CreateObservation(ctx, obs); err != nil {
		return err
	}
	if f.objectGateway != nil {
		changed, err := f.syncObservationIdentityHistory(ctx, obs, nil, obs.StartedAt)
		if err != nil {
			return err
		}
		if changed {
			obs.UpdatedAt = time.Now().UTC()
			if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
				return err
			}
		}
	}
	publishObservation(ctx, f.publisher, "created", obs)
	return nil
}

func (f ObservationFunctions) GetObservation(ctx context.Context, observationID string) (*model.Observation, error) {
	if observationID == "" {
		return nil, model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	return f.pgStore.GetObservation(ctx, observationID)
}

func (f ObservationFunctions) ListObservations(ctx context.Context, params store.ObservationListParams) (store.ObservationListResult, error) {
	return f.pgStore.ListObservations(ctx, params)
}

func (f ObservationFunctions) UpdateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs, false); err != nil {
		return err
	}
	if err := validateObservationJSONStructure(obs.JSON); err != nil {
		return err
	}
	if err := rejectClientHistoryObjectID(obs.JSON); err != nil {
		return err
	}
	existing, err := f.pgStore.GetObservation(ctx, obs.ObservationID)
	if err != nil {
		return err
	}
	if err := rejectIdentityRemovalWithoutTelemetry(existing.JSON, obs.JSON); err != nil {
		return err
	}
	// Reject extra-only incoming patches on rows that already satisfy the section
	// requirement. Legacy extra-only rows cannot receive another extra-only patch.
	if err := validateObservationJSONRequiresSection(obs.JSON); err != nil {
		if err := validateObservationJSONRequiresSection(existing.JSON); err != nil {
			return err
		}
	}
	obs.JSON, err = applyLatestTelemetryMutationRules(existing.JSON, obs.JSON)
	if err != nil {
		return err
	}
	incomingPatch, err := observationJSONPatchMap(obs.JSON)
	if err != nil {
		return err
	}
	previewJSON, err := mergeObservationJSON(existing.JSON, incomingPatch)
	if err != nil {
		return err
	}
	preview := *obs
	preview.JSON = previewJSON
	if issues := f.protoValidator.ValidateObservation(&preview); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if obs.StartedAt.IsZero() {
		obs.StartedAt = existing.StartedAt
	}
	if err := endedAtOrderingValid(obs.StartedAt, obs.EndedAt); err != nil {
		return err
	}
	storeJSON, err := observationJSONForInitialStore(existing, obs.JSON)
	if err != nil {
		return err
	}
	storeObs := *obs
	storeObs.JSON = storeJSON
	obs.UpdatedAt = time.Now().UTC()
	storeObs.UpdatedAt = obs.UpdatedAt
	f.log.InfoContext(ctx, "observation", "updating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpdateObservation(ctx, &storeObs); err != nil {
		return err
	}
	if f.objectGateway != nil {
		effectiveAt := observationIdentityEffectiveAt(obs, time.Now().UTC())
		changed, err := f.syncObservationIdentityHistory(ctx, obs, existing, effectiveAt)
		if err != nil {
			return err
		}
		if changed {
			obs.UpdatedAt = time.Now().UTC()
			if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
				return err
			}
		}
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return nil
}

func (f ObservationFunctions) DeleteObservation(ctx context.Context, observationID string) error {
	if observationID == "" {
		return model.NewFieldError("INVALID_INPUT", "observation_id is required", "observation_id")
	}
	f.log.InfoContext(ctx, "observation", "deleting observation", logging.String("observation_id", observationID))
	observation, err := f.pgStore.GetObservation(ctx, observationID)
	if err != nil {
		return err
	}
	if err := f.pgStore.DeleteObservation(ctx, observationID); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "deleted", observation)
	return nil
}

func (f ObservationFunctions) UpsertObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs, true); err != nil {
		return err
	}
	if err := validateObservationJSONStructure(obs.JSON); err != nil {
		return err
	}
	if err := rejectClientHistoryObjectID(obs.JSON); err != nil {
		return err
	}
	existing, existingErr := f.pgStore.GetObservation(ctx, obs.ObservationID)
	if existingErr != nil && !errors.Is(existingErr, model.ErrNotFound) {
		return existingErr
	}
	if existingErr == nil {
		if err := rejectIdentityRemovalWithoutTelemetry(existing.JSON, obs.JSON); err != nil {
			return err
		}
		// Reject extra-only incoming patches on rows that already satisfy the section
		// requirement. Legacy extra-only rows cannot receive another extra-only patch.
		if err := validateObservationJSONRequiresSection(obs.JSON); err != nil {
			if err := validateObservationJSONRequiresSection(existing.JSON); err != nil {
				return err
			}
		}
		if observationJSONHasKey(obs.JSON, "latest_telemetry") {
			return model.NewFieldError("INVALID_INPUT", "latest_telemetry must be updated through ingest", "json.latest_telemetry")
		}
		var err error
		obs.JSON, err = applyLatestTelemetryMutationRules(existing.JSON, obs.JSON)
		if err != nil {
			return err
		}
		if obs.StartedAt.IsZero() {
			obs.StartedAt = existing.StartedAt
		}
		incomingPatch, err := observationJSONPatchMap(obs.JSON)
		if err != nil {
			return err
		}
		previewJSON, err := mergeObservationJSON(existing.JSON, incomingPatch)
		if err != nil {
			return err
		}
		preview := *obs
		preview.JSON = previewJSON
		if issues := f.protoValidator.ValidateObservation(&preview); len(issues) > 0 {
			return protocolvalidation.NewValidationError(issues)
		}
	} else {
		if err := validateObservationJSONRequiresSection(obs.JSON); err != nil {
			return err
		}
		if observationJSONHasKey(obs.JSON, "latest_telemetry") {
			return model.NewFieldError("INVALID_INPUT", "latest_telemetry must be updated through ingest", "json.latest_telemetry")
		}
		if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
			return protocolvalidation.NewValidationError(issues)
		}
	}
	if err := endedAtOrderingValid(obs.StartedAt, obs.EndedAt); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	storeObs := *obs
	if existingErr == nil {
		storeJSON, err := observationJSONForInitialStore(existing, obs.JSON)
		if err != nil {
			return err
		}
		storeObs.JSON = storeJSON
	}
	f.log.InfoContext(ctx, "observation", "upserting observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpsertObservation(ctx, &storeObs); err != nil {
		return err
	}
	if f.objectGateway != nil {
		effectiveAt := observationIdentityEffectiveAt(obs, obs.StartedAt)
		if effectiveAt.IsZero() {
			effectiveAt = now
		}
		var existingObs *model.Observation
		if existingErr == nil {
			existingObs = existing
		}
		changed, err := f.syncObservationIdentityHistory(ctx, obs, existingObs, effectiveAt)
		if err != nil {
			return err
		}
		if changed {
			obs.UpdatedAt = time.Now().UTC()
			if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
				return err
			}
		}
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return nil
}

func (f ObservationFunctions) IngestObservationTelemetry(ctx context.Context, ingest ObservationTelemetryIngest) (*model.Observation, error) {
	if f.objectGateway == nil {
		return nil, model.NewFieldError("INTERNAL", "observation object gateway is not configured", "object_gateway")
	}
	if f.entityStore == nil {
		return nil, model.NewFieldError("INTERNAL", "observation entity store is not configured", "entity_store")
	}
	telemetry, err := canonicalizeTelemetryJSON(ingest.TelemetryJSON)
	if err != nil {
		return nil, err
	}
	observedAt, err := telemetry.observedAt()
	if err != nil {
		return nil, err
	}

	obs, err := f.pgStore.GetObservation(ctx, ingest.ObservationID)
	creating := false
	if err != nil {
		if err != model.ErrNotFound {
			return nil, err
		}
		if ingest.StartedAt.IsZero() {
			return nil, model.NewFieldError("INVALID_INPUT", "started_at is required when creating a missing observation", "started_at")
		}
		creating = true
		obs = &model.Observation{
			ObservationID:  ingest.ObservationID,
			SourceAssetID:  ingest.SourceAssetID,
			TargetEntityID: ingest.TargetEntityID,
			StartedAt:      ingest.StartedAt.UTC(),
			EndedAt:        ingest.EndedAt,
		}
		if err := validateObservationModel(obs, true); err != nil {
			return nil, err
		}
		if err := f.validateObservationIngestRefs(ctx, obs); err != nil {
			return nil, err
		}
		if err := endedAtOrderingValid(obs.StartedAt, obs.EndedAt); err != nil {
			return nil, err
		}
	} else {
		if err := f.validateObservationIngestRefs(ctx, obs); err != nil {
			return nil, err
		}
		if ingest.EndedAt != nil {
			obs.EndedAt = ingest.EndedAt
		}
		if err := endedAtOrderingValid(obs.StartedAt, obs.EndedAt); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	historyObjectID, err := f.ensureObservationHistoryObject(ctx, obs.ObservationID, now)
	if err != nil {
		return nil, err
	}

	if creating {
		obs.CreatedAt = now
		obs.UpdatedAt = now
		obs.JSON, err = mergeObservationJSON(obs.JSON, map[string]any{"history_object_id": historyObjectID})
		if err != nil {
			return nil, err
		}
		if identity, err := parseIdentityBytes(ingest.IdentityJSON); err != nil {
			return nil, err
		} else if len(identity) > 0 {
			obs.JSON, err = mergeObservationJSON(obs.JSON, map[string]any{"identity": identity})
			if err != nil {
				return nil, err
			}
		}
	} else {
		obs.JSON, err = mergeObservationJSON(obs.JSON, map[string]any{"history_object_id": historyObjectID})
		if err != nil {
			return nil, err
		}
	}

	eventLine, err := buildTelemetryHistoryLine(obs.ObservationID, obs.Version, telemetry, now)
	if err != nil {
		return nil, err
	}
	if creating || telemetryObservedAtIsNewer(obs, observedAt) {
		if err := applyTelemetryEventToObservation(obs, telemetry, observedAt); err != nil {
			return nil, err
		}
	}
	obs.UpdatedAt = now

	if creating {
		if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
			return nil, protocolvalidation.NewValidationError(issues)
		}
		if err := f.pgStore.CreateObservation(ctx, obs); err != nil {
			return nil, err
		}
		if identity, hasIdentity, err := parseObservationIdentity(obs.JSON); err != nil {
			return nil, err
		} else if hasIdentity {
			if err := f.appendIdentityPatchIfNeeded(ctx, obs, nil, identity, obs.StartedAt); err != nil {
				return nil, err
			}
			obs.UpdatedAt = time.Now().UTC()
			if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
				return nil, err
			}
		}
		if err := f.appendTelemetryHistoryIfNeeded(ctx, historyObjectID, eventLine); err != nil {
			return nil, err
		}
		publishObservation(ctx, f.publisher, "created", obs)
		return obs, nil
	}

	if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
		if !errors.Is(err, model.ErrVersionConflict) {
			return nil, err
		}
		if reconcileErr := f.reconcileAfterHistoryAppend(ctx, obs, historyObjectID, eventLine); reconcileErr != nil {
			return nil, reconcileErr
		}
		obs, err = f.pgStore.GetObservation(ctx, obs.ObservationID)
		if err != nil {
			return nil, err
		}
	}
	if err := f.appendTelemetryHistoryIfNeeded(ctx, historyObjectID, eventLine); err != nil {
		return nil, err
	}
	if identity, err := parseIdentityBytes(ingest.IdentityJSON); err != nil {
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
			if err := f.appendIdentityPatchIfNeeded(ctx, obs, prev, identity, observedAt); err != nil {
				return nil, err
			}
			obs.UpdatedAt = time.Now().UTC()
			if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
				return nil, err
			}
		}
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return obs, nil
}

func mustEventID(line []byte) string {
	id, err := historyEventIDFromLine(line)
	if err != nil {
		return ""
	}
	return id
}

func (f ObservationFunctions) validateObservationIngestRefs(ctx context.Context, obs *model.Observation) error {
	source, err := f.entityStore.GetEntity(ctx, obs.SourceAssetID)
	if err != nil {
		return err
	}
	if source.Type != model.EntityTypeAsset {
		return model.NewFieldError("INVALID_INPUT", "source_asset_id must reference an asset entity", "source_asset_id")
	}
	if obs.TargetEntityID == nil {
		return nil
	}
	target, err := f.entityStore.GetEntity(ctx, *obs.TargetEntityID)
	if err != nil {
		return err
	}
	if target.Type != model.EntityTypeTrack && target.Type != model.EntityTypeGeofeature {
		return model.NewFieldError("INVALID_INPUT", "target_entity_id must reference a track or geofeature entity", "target_entity_id")
	}
	return nil
}

func ObservationHistoryObjectID(observationID string) string {
	sum := sha256.Sum256([]byte(observationID))
	return "obs_hist_" + hex.EncodeToString(sum[:])[:32]
}

func validateObservationHistoryObject(obj *model.Object, observationID string) error {
	if obj.Type == model.ObjectTypeObservationHistory && obj.OwnerType == model.OwnerTypeObservation && obj.OwnerID == observationID {
		return nil
	}
	return model.NewFieldError(
		model.ErrConflict.Code,
		fmt.Sprintf("history object %q must be type %q owned by observation %q", obj.ObjectID, model.ObjectTypeObservationHistory, observationID),
		"history_object_id",
	)
}
