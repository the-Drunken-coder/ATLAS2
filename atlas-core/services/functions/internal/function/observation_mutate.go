package function

import (
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
)

// preparedExistingObservationMutation is the result of validating and merging an update/upsert patch.
type preparedExistingObservationMutation struct {
	StoreObs *model.Observation
}

// prepareExistingObservationMutation validates and merges an incoming patch against an existing row.
// The returned StoreObs is ready for the first DB write; obs may have StartedAt filled from existing.
func (f ObservationFunctions) prepareExistingObservationMutation(existing, obs *model.Observation) (preparedExistingObservationMutation, error) {
	if err := rejectIdentityRemovalWithoutTelemetry(existing.JSON, obs.JSON); err != nil {
		return preparedExistingObservationMutation{}, err
	}
	if err := validateObservationJSONRequiresSection(obs.JSON); err != nil {
		if err := validateObservationJSONRequiresSection(existing.JSON); err != nil {
			return preparedExistingObservationMutation{}, err
		}
	}
	var err error
	obs.JSON, err = applyLatestTelemetryMutationRules(existing.JSON, obs.JSON)
	if err != nil {
		return preparedExistingObservationMutation{}, err
	}
	if obs.StartedAt.IsZero() {
		obs.StartedAt = existing.StartedAt
	}
	incomingPatch, err := observationJSONPatchMap(obs.JSON)
	if err != nil {
		return preparedExistingObservationMutation{}, err
	}
	previewJSON, err := mergeObservationJSON(existing.JSON, incomingPatch)
	if err != nil {
		return preparedExistingObservationMutation{}, err
	}
	preview := *obs
	preview.JSON = previewJSON
	if issues := f.protoValidator.ValidateObservation(&preview); len(issues) > 0 {
		return preparedExistingObservationMutation{}, protocolvalidation.NewValidationError(issues)
	}
	storeJSON, err := observationJSONForInitialStore(existing, previewJSON)
	if err != nil {
		return preparedExistingObservationMutation{}, err
	}
	storeObs := *obs
	storeObs.JSON = storeJSON
	return preparedExistingObservationMutation{StoreObs: &storeObs}, nil
}
