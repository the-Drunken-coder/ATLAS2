package sim

import (
	"testing"
	"time"
)

func TestADSBFeedClampsNegativeDelay(t *testing.T) {
	rng := newRNG(1)
	feed, err := newADSBFeed(FeedConfig{
		Type:          "adsb",
		ObservationID: "obs_adsb",
		SourceAssetID: "asset_adsb",
		IntervalMS:    500,
		DelayMSMin:    -200,
		DelayMSMax:    -50,
	}, rng)
	if err != nil {
		t.Fatalf("newADSBFeed: %v", err)
	}
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	snap, err := feed.run(start, time.Second, Motion{
		InitialLat:  40.0,
		InitialLon:  -74.0,
		InitialAltM: 1000,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if snap.UpdatedAt.Before(snap.LatestTelemetryAt) {
		t.Fatalf("UpdatedAt must not precede LatestTelemetryAt: latest=%s updated=%s",
			snap.LatestTelemetryAt, snap.UpdatedAt)
	}
}

func TestCameraLOBFeedClampsNegativeDelay(t *testing.T) {
	rng := newRNG(2)
	feed, err := newCameraLOBFeed(FeedConfig{
		Type:          "camera_lob",
		ObservationID: "obs_cam",
		SourceAssetID: "asset_cam",
		IntervalMS:    1000,
		DelayMSMin:    -100,
		DelayMSMax:    -10,
		ObserverLat:   40.01,
		ObserverLon:   -74.01,
		ObserverAltM:  20,
	}, rng)
	if err != nil {
		t.Fatalf("newCameraLOBFeed: %v", err)
	}
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	snap, err := feed.run(start, 2*time.Second, Motion{
		InitialLat:  40.0,
		InitialLon:  -74.0,
		InitialAltM: 1000,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if snap.UpdatedAt.Before(snap.LatestTelemetryAt) {
		t.Fatalf("UpdatedAt must not precede LatestTelemetryAt: latest=%s updated=%s",
			snap.LatestTelemetryAt, snap.UpdatedAt)
	}
}

func TestCameraLOBFeedNormalizesInvertedDelayBounds(t *testing.T) {
	rng := newRNG(3)
	feed, err := newCameraLOBFeed(FeedConfig{
		Type:          "camera_lob",
		ObservationID: "obs_cam",
		SourceAssetID: "asset_cam",
		IntervalMS:    1000,
		DelayMSMin:    200,
		DelayMSMax:    50,
		ObserverLat:   40.01,
		ObserverLon:   -74.01,
		ObserverAltM:  20,
	}, rng)
	if err != nil {
		t.Fatalf("newCameraLOBFeed: %v", err)
	}
	if feed.cfg.DelayMSMin != 200 || feed.cfg.DelayMSMax != 200 {
		t.Fatalf("expected inverted bounds normalized to 200..200, got %d..%d", feed.cfg.DelayMSMin, feed.cfg.DelayMSMax)
	}
}
