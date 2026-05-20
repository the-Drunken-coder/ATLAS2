package atlasio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/model"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
)

const ProvenanceFilename = "fusion-provenance.ndjson"

type Sink struct {
	Client functionsv1.AtlasFunctionsServiceClient
	Now    func() time.Time
}

func (s Sink) Commit(ctx context.Context, result core.Result) error {
	if s.Client == nil {
		return fmt.Errorf("atlas functions client is required")
	}
	for _, update := range result.TrackUpdates {
		if err := s.upsertTrack(ctx, update); err != nil {
			return err
		}
	}
	for _, record := range result.Provenance {
		if err := s.appendProvenance(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (s Sink) upsertTrack(ctx context.Context, update core.TrackUpdate) error {
	now := s.now()
	entity := &model.Entity{
		EntityID:  update.TrackID,
		Type:      model.EntityTypeTrack,
		JSON:      append([]byte(nil), update.JSON...),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.Client.UpsertEntity(ctx, &sharedv1.EntityRequest{Entity: pbconv.EntityToProto(entity)})
	if err != nil {
		return fmt.Errorf("upsert track %q: %w", update.TrackID, err)
	}
	return nil
}

func (s Sink) appendProvenance(ctx context.Context, record core.ProvenanceRecord) error {
	objectID := trackProvenanceObjectID(record.TrackID)
	now := s.now()
	object := &model.Object{
		ObjectID:  objectID,
		Type:      model.ObjectTypeTrackProvenance,
		OwnerType: model.OwnerTypeEntity,
		OwnerID:   record.TrackID,
		JSON:      []byte(`{"format_version":"v1"}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.Client.UpsertObject(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)}); err != nil {
		return fmt.Errorf("upsert track provenance object %q: %w", objectID, err)
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode track provenance for %q: %w", record.TrackID, err)
	}
	line = append(line, '\n')
	currentSize, err := s.currentFileSize(ctx, objectID, ProvenanceFilename)
	if err != nil {
		return err
	}
	stream, err := s.Client.AppendObjectFile(ctx)
	if err != nil {
		return fmt.Errorf("open provenance append stream %q: %w", objectID, err)
	}
	if err := stream.Send(&sharedv1.AppendFileChunk{
		ObjectId:            objectID,
		Filename:            ProvenanceFilename,
		Data:                line,
		FinalChunk:          true,
		ExpectedSize:        currentSize + int64(len(line)),
		CurrentExpectedSize: currentSize,
	}); err != nil {
		_ = stream.CloseSend()
		return fmt.Errorf("send provenance append %q: %w", objectID, err)
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("commit provenance append %q: %w", objectID, err)
	}
	return nil
}

func (s Sink) currentFileSize(ctx context.Context, objectID, filename string) (int64, error) {
	resp, err := s.Client.GetObjectManifest(ctx, &sharedv1.GetObjectManifestRequest{ObjectId: objectID})
	if err != nil {
		return 0, fmt.Errorf("read provenance manifest %q: %w", objectID, err)
	}
	info := resp.GetManifest().GetFiles()[filename]
	if info == nil {
		return 0, nil
	}
	return info.GetSize(), nil
}

func (s Sink) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func trackProvenanceObjectID(trackID string) string {
	return "fusion_prov_" + trackID
}
