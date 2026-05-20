package atlasio

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
	if err := s.appendProvenanceBatch(ctx, result.Provenance); err != nil {
		return err
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

func (s Sink) appendProvenanceBatch(ctx context.Context, records []core.ProvenanceRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Group records by track ID for efficient batching
	byTrack := make(map[string][]core.ProvenanceRecord)
	for _, record := range records {
		byTrack[record.TrackID] = append(byTrack[record.TrackID], record)
	}

	for trackID, trackRecords := range byTrack {
		if err := s.appendProvenanceForTrack(ctx, trackID, trackRecords); err != nil {
			return err
		}
	}
	return nil
}

func (s Sink) appendProvenanceForTrack(ctx context.Context, trackID string, records []core.ProvenanceRecord) error {
	objectID := trackProvenanceObjectID(trackID)
	now := s.now()
	object := &model.Object{
		ObjectID:  objectID,
		Type:      model.ObjectTypeTrackProvenance,
		OwnerType: model.OwnerTypeEntity,
		OwnerID:   trackID,
		JSON:      []byte(`{"format_version":"v1"}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.Client.UpsertObject(ctx, &sharedv1.ObjectRequest{Object: pbconv.ObjectToProto(object)}); err != nil {
		return fmt.Errorf("upsert track provenance object %q: %w", objectID, err)
	}

	// Read existing provenance records to check for duplicates
	existingRecords, err := s.readExistingProvenance(ctx, objectID)
	if err != nil {
		return err
	}

	// Build map of existing record signatures for deduplication
	existing := make(map[string]bool)
	for _, record := range existingRecords {
		sig := provenanceSignature(record)
		existing[sig] = true
	}

	// Filter out records that already exist
	var newRecords []core.ProvenanceRecord
	for _, record := range records {
		sig := provenanceSignature(record)
		if !existing[sig] {
			newRecords = append(newRecords, record)
		}
	}

	if len(newRecords) == 0 {
		return nil // All records already exist
	}

	// Marshal all new records into a single batch
	var batchData []byte
	for _, record := range newRecords {
		line, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode track provenance for %q: %w", trackID, err)
		}
		batchData = append(batchData, line...)
		batchData = append(batchData, '\n')
	}

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
		Data:                batchData,
		FinalChunk:          true,
		ExpectedSize:        currentSize + int64(len(batchData)),
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

func (s Sink) appendProvenance(ctx context.Context, record core.ProvenanceRecord) error {
	return s.appendProvenanceForTrack(ctx, record.TrackID, []core.ProvenanceRecord{record})
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

func (s Sink) readExistingProvenance(ctx context.Context, objectID string) ([]core.ProvenanceRecord, error) {
	stream, err := s.Client.ReadObjectFile(ctx, &sharedv1.ReadFileRequest{
		ObjectId:  objectID,
		Filename:  ProvenanceFilename,
		ChunkSize: 1024 * 1024, // 1MB chunks
	})
	if err != nil {
		// If file doesn't exist yet, return empty list
		return nil, nil
	}

	var fileData []byte
	for {
		chunk, err := stream.Recv()
		if err != nil {
			// If we get an error reading (e.g., file not found), return empty list
			return nil, nil
		}
		fileData = append(fileData, chunk.GetData()...)
		if chunk.GetFinalChunk() {
			break
		}
	}

	var records []core.ProvenanceRecord
	for len(fileData) > 0 {
		idx := 0
		for idx < len(fileData) && fileData[idx] != '\n' {
			idx++
		}
		if idx == 0 {
			break
		}
		line := fileData[:idx]
		if idx+1 <= len(fileData) {
			fileData = fileData[idx+1:]
		} else {
			fileData = nil
		}

		var record core.ProvenanceRecord
		if err := json.Unmarshal(line, &record); err != nil {
			// Skip malformed lines
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func provenanceSignature(record core.ProvenanceRecord) string {
	// Create a unique signature based on the combination of fields that identify a unique provenance record
	// Using CreatedAt timestamp (nanosecond precision) + TrackID + EngineName + EngineVersion + sorted input refs
	var inputSigs []string
	for _, input := range record.Inputs {
		inputSigs = append(inputSigs, fmt.Sprintf("%s:%d:%d", input.ObservationID, input.Version, input.ObservedAt.UnixNano()))
	}
	sort.Strings(inputSigs)
	return fmt.Sprintf("%s:%s:%s:%s:%d:%s",
		record.TrackID,
		record.EngineName,
		record.EngineVersion,
		record.CreatedAt.Format(time.RFC3339Nano),
		len(inputSigs),
		strings.Join(inputSigs, ","))
}

func trackProvenanceObjectID(trackID string) string {
	return "fusion_prov_" + trackID
}
