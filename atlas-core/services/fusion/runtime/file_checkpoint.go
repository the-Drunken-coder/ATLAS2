package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	var checkpoint core.Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return core.Checkpoint{}, fmt.Errorf("parse fusion checkpoint: %w", err)
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
