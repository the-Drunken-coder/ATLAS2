package function

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

const ObservationSightingsFilename = "sightings.ndjson"

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

type ObservationSightingIngest struct {
	ObservationID  string
	SourceAssetID  string
	TargetEntityID *string
	SightingJSON   []byte
}

func (f ObservationFunctions) CreateObservation(ctx context.Context, obs *model.Observation) error {
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if err := deriveObservationFields(obs); err != nil {
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
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if err := deriveObservationFields(obs); err != nil {
		return err
	}
	obs.UpdatedAt = time.Now().UTC()
	f.log.InfoContext(ctx, "observation", "updating observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpdateObservation(ctx, obs); err != nil {
		return err
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
	if err := validateObservationModel(obs); err != nil {
		return err
	}
	if obs.JSON == nil {
		obs.JSON = []byte("{}")
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return protocolvalidation.NewValidationError(issues)
	}
	if err := deriveObservationFields(obs); err != nil {
		return err
	}
	now := time.Now().UTC()
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = now
	}
	obs.UpdatedAt = now
	f.log.InfoContext(ctx, "observation", "upserting observation", logging.String("observation_id", obs.ObservationID), logging.String("source_asset_id", obs.SourceAssetID))
	if err := f.pgStore.UpsertObservation(ctx, obs); err != nil {
		return err
	}
	publishObservation(ctx, f.publisher, "updated", obs)
	return nil
}

func (f ObservationFunctions) IngestObservationSighting(ctx context.Context, ingest ObservationSightingIngest) (*model.Observation, error) {
	if f.objectGateway == nil {
		return nil, model.NewFieldError("INTERNAL", "observation object gateway is not configured", "object_gateway")
	}
	if f.entityStore == nil {
		return nil, model.NewFieldError("INTERNAL", "observation entity store is not configured", "entity_store")
	}
	historyObjectID := ObservationHistoryObjectID(ingest.ObservationID)
	sightingID := generateSightingID(ingest.ObservationID, ingest.SightingJSON)
	obsJSON, sightingLine, err := observationJSONForIngest(historyObjectID, sightingID, ingest.SightingJSON)
	if err != nil {
		return nil, err
	}
	obs := &model.Observation{
		ObservationID:  ingest.ObservationID,
		SourceAssetID:  ingest.SourceAssetID,
		TargetEntityID: ingest.TargetEntityID,
		JSON:           obsJSON,
	}
	if err := validateObservationModel(obs); err != nil {
		return nil, err
	}
	if issues := f.protoValidator.ValidateObservation(obs); len(issues) > 0 {
		return nil, protocolvalidation.NewValidationError(issues)
	}
	if err := deriveObservationFields(obs); err != nil {
		return nil, err
	}
	if err := f.validateObservationIngestRefs(ctx, obs); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	historyObject := &model.Object{
		ObjectID:  historyObjectID,
		Type:      model.ObjectTypeObservationHistory,
		OwnerType: model.OwnerTypeObservation,
		OwnerID:   obs.ObservationID,
		JSON:      []byte(`{"format_version":"v1"}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if issues := f.protoValidator.ValidateObject(historyObject); len(issues) > 0 {
		return nil, protocolvalidation.NewValidationError(issues)
	}
	if err := f.objectGateway.EnsureObjectCreated(ctx, historyObject); err != nil {
		return nil, err
	}
	if _, err := f.objectGateway.AppendFile(ctx, historyObjectID, ObservationSightingsFilename, sightingLine); err != nil {
		return nil, err
	}
	if err := f.UpsertObservation(ctx, obs); err != nil {
		return nil, err
	}
	return obs, nil
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

// generateSightingID produces a deterministic unique identifier for a sighting.
// This ID enables idempotent ingestion: if the same sighting is appended multiple times
// (e.g., due to retry after UpsertObservation failure), consumers can deduplicate by sighting_id.
func generateSightingID(observationID string, sightingJSON []byte) string {
	h := sha256.New()
	h.Write([]byte(observationID))
	h.Write([]byte{0}) // separator
	h.Write(sightingJSON)
	sum := h.Sum(nil)
	return fmt.Sprintf("sighting_%s", hex.EncodeToString(sum[:16]))
}

func observationJSONForIngest(historyObjectID string, sightingID string, sightingJSON []byte) ([]byte, []byte, error) {
	var sighting any
	if err := json.Unmarshal(sightingJSON, &sighting); err != nil {
		return nil, nil, model.NewFieldError("INVALID_INPUT", "sighting must be valid JSON", "sighting")
	}
	sightingObject, ok := sighting.(map[string]any)
	if !ok {
		return nil, nil, model.NewFieldError("INVALID_INPUT", "sighting must be a JSON object", "sighting")
	}
	// Create a copy of the sighting object with sighting_id added for idempotency.
	// Consumers reading sightings.ndjson MUST deduplicate by this field.
	sightingWithID := make(map[string]any, len(sightingObject)+1)
	for k, v := range sightingObject {
		sightingWithID[k] = v
	}
	sightingWithID["sighting_id"] = sightingID
	compactSighting, err := json.Marshal(sightingWithID)
	if err != nil {
		return nil, nil, model.NewFieldError("INVALID_INPUT", "sighting must be valid JSON", "sighting")
	}
	// Use the original sightingObject (without sighting_id) for latest_sighting
	observationJSON, err := json.Marshal(map[string]any{
		"state":               "active",
		"latest_sighting":     sightingObject,
		"sightings_object_id": historyObjectID,
	})
	if err != nil {
		return nil, nil, model.NewFieldError("INVALID_INPUT", "sighting must be valid JSON", "sighting")
	}
	return observationJSON, append(bytes.TrimSpace(compactSighting), '\n'), nil
}

func deriveObservationFields(obs *model.Observation) error {
	var root struct {
		LatestSighting *struct {
			ObservedAt string `json:"observed_at"`
		} `json:"latest_sighting"`
	}
	if err := json.Unmarshal(obs.JSON, &root); err != nil {
		return model.NewFieldError("INVALID_INPUT", "observation json must be a JSON object", "json")
	}
	obs.ObservedAt = nil
	if root.LatestSighting == nil || root.LatestSighting.ObservedAt == "" {
		return nil
	}
	observedAt, err := time.Parse(time.RFC3339, root.LatestSighting.ObservedAt)
	if err != nil {
		return model.NewFieldError("INVALID_INPUT", "latest_sighting.observed_at must be RFC 3339", "json.latest_sighting.observed_at")
	}
	utc := observedAt.UTC()
	obs.ObservedAt = &utc
	return nil
}
