package service

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/services/functions/internal/service/testutil"
)

func TestIngestObservationTelemetryNilRequestDoesNotPanic(t *testing.T) {
	var srv *Server
	_ = newFunctionsTestEnv(t, testutil.NewFakeDataStorage(), func(s *Server) { srv = s })
	_, err := srv.IngestObservationTelemetry(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}
