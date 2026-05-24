package testutil

import (
	"context"
	"testing"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/rpcerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFakeDataStorageUpdateObjectRejectsStaleVersion(t *testing.T) {
	fake := NewFakeDataStorage()
	now := timestamppb.Now()
	fake.Objects["obj_001"] = &sharedv1.Object{
		ObjectId:  "obj_001",
		Type:      "log",
		Version:   2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := fake.UpdateObject(context.Background(), &sharedv1.ObjectRequest{
		Object: &sharedv1.Object{
			ObjectId:  "obj_001",
			Type:      "log",
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		},
	})
	if err == nil {
		t.Fatal("expected stale update to fail")
	}
	if status.Code(rpcerrors.ToStatus(err)) != codes.Aborted {
		t.Fatalf("expected Aborted for version conflict, got %v", err)
	}
}

func TestFakeDataStorageUpdateObservationRejectsStaleVersion(t *testing.T) {
	fake := NewFakeDataStorage()
	now := timestamppb.Now()
	startedAt := timestamppb.New(now.AsTime())
	fake.Observations["obs_001"] = &sharedv1.Observation{
		ObservationId: "obs_001",
		SourceAssetId: "asset_001",
		StartedAt:     startedAt,
		Version:       2,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := fake.UpdateObservation(context.Background(), &sharedv1.ObservationRequest{
		Observation: &sharedv1.Observation{
			ObservationId: "obs_001",
			SourceAssetId: "asset_001",
			StartedAt:     startedAt,
			Version:       1,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	})
	if err == nil {
		t.Fatal("expected stale update to fail")
	}
	if status.Code(rpcerrors.ToStatus(err)) != codes.Aborted {
		t.Fatalf("expected Aborted for version conflict, got %v", err)
	}
}
