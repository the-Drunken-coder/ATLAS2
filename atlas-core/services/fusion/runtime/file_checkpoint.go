package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anomalyco/atlas-core/services/fusion/core"
)

type FileCheckpointStore struct {
	Path string
}

func (s FileCheckpointStore) Load(context.Context) (core.Checkpoint, error) {
	if s.Path == "" {
		return core.Checkpoint{}, nil
	}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return core.Checkpoint{}, nil
	}
	if err != nil {
		return core.Checkpoint{}, fmt.Errorf("read fusion checkpoint: %w", err)
	}
	checkpoint, err := decodeCheckpoint(data)
	if err != nil {
		return core.Checkpoint{}, err
	}
	return checkpoint, nil
}

type checkpointFile struct {
	core.Checkpoint
	ObservedAtLegacy time.Time `json:"observed_at,omitempty"`
}

func decodeCheckpoint(data []byte) (core.Checkpoint, error) {
	var file checkpointFile
	if err := json.Unmarshal(data, &file); err != nil {
		return core.Checkpoint{}, fmt.Errorf("parse fusion checkpoint: %w", err)
	}
	checkpoint := file.Checkpoint
	if checkpoint.UpdatedAt.IsZero() && !file.ObservedAtLegacy.IsZero() {
		checkpoint.UpdatedAt = file.ObservedAtLegacy.UTC()
	}
	return checkpoint, nil
}

func (s FileCheckpointStore) Save(_ context.Context, checkpoint core.Checkpoint) error {
	if s.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create fusion checkpoint dir: %w", err)
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fusion checkpoint: %w", err)
	}
	return os.WriteFile(s.Path, append(data, '\n'), 0o600)
}
