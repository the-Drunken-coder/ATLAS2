package multisensor

import (
	"testing"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

func TestFusedTrackIDDistinguishesLOBOnlyBatches(t *testing.T) {
	lobJSON := []byte(`{"latest_telemetry":{"kind":"line_of_bearing","data":{"observer_latitude":40.71,"observer_longitude":-74.01,"azimuth_deg":101,"elevation_deg":15}}}`)
	idA := fusedTrackID([]core.ObservationInput{
		{ObservationID: "obs_cam_north", JSON: lobJSON},
		{ObservationID: "obs_cam_south", JSON: lobJSON},
	})
	idB := fusedTrackID([]core.ObservationInput{
		{ObservationID: "obs_cam_east", JSON: lobJSON},
		{ObservationID: "obs_cam_west", JSON: lobJSON},
	})
	if idA == idB {
		t.Fatalf("LOB-only batches with different observation IDs must not share track ID: %q", idA)
	}
	if idA == "" || idB == "" {
		t.Fatal("expected non-empty track IDs")
	}
}

func TestFusedTrackIDNoDelimiterCollision(t *testing.T) {
	lobJSON := []byte(`{"latest_telemetry":{"kind":"line_of_bearing","data":{"observer_latitude":40.71,"observer_longitude":-74.01,"azimuth_deg":101,"elevation_deg":15}}}`)
	idABc := fusedTrackID([]core.ObservationInput{
		{ObservationID: "ab", JSON: lobJSON},
		{ObservationID: "c", JSON: lobJSON},
	})
	idAbc := fusedTrackID([]core.ObservationInput{
		{ObservationID: "a", JSON: lobJSON},
		{ObservationID: "bc", JSON: lobJSON},
	})
	if idABc == idAbc {
		t.Fatalf("LOB-only track IDs must not collide across ID boundaries: %q", idABc)
	}
}

func TestFusedTrackIDStableForSameLOBOnlyBatch(t *testing.T) {
	lobJSON := []byte(`{"latest_telemetry":{"kind":"line_of_bearing","data":{"observer_latitude":40.71,"observer_longitude":-74.01,"azimuth_deg":101,"elevation_deg":15}}}`)
	obs := []core.ObservationInput{
		{ObservationID: "obs_cam_north", JSON: lobJSON},
		{ObservationID: "obs_cam_south", JSON: lobJSON},
	}
	id1 := fusedTrackID(obs)
	id2 := fusedTrackID(obs)
	if id1 != id2 {
		t.Fatal("expected deterministic track ID for the same batch")
	}
}
