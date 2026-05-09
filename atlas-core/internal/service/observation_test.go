package service

import (
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

func TestObservationFunctions_ValidateObservationID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{SourceAssetID: "asset_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty observation_id")
	}
}

func TestObservationFunctions_ValidateSourceAssetID(t *testing.T) {
	f := ObservationFunctions{}
	obs := &model.Observation{ObservationID: "obs_001", JSON: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := f.CreateObservation(nil, obs); err == nil {
		t.Fatal("expected error for empty source_asset_id")
	}
}
