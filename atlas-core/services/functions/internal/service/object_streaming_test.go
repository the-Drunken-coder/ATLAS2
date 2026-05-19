package service

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/logging"
)

func TestBestEffortPublishObjectUpdatedNoOpOnNilError(t *testing.T) {
	bestEffortPublishObjectUpdated(context.Background(), logging.New("debug", "atlas-test", "test"), "obj_001", nil)
}

func TestBestEffortPublishObjectUpdatedNoPanicOnErrorWithNilLogger(t *testing.T) {
	bestEffortPublishObjectUpdated(context.Background(), nil, "obj_001", errors.New("publish failed"))
}
