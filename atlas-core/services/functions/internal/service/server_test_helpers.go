package service

import (
	"testing"

	"github.com/anomalyco/atlas-core/services/functions/internal/changefeed"
	"github.com/anomalyco/atlas-core/services/functions/internal/datastorageclient"
	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	"github.com/anomalyco/atlas-core/services/functions/internal/service/testutil"
	datastoragev1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/datastorage/v1"
	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/protocolvalidation"
	"google.golang.org/grpc"
)

type functionsTestEnv struct {
	Client     functionsv1.AtlasFunctionsServiceClient
	Changefeed functionsv1.ChangefeedServiceClient
	Fake       *testutil.FakeDataStorage
	Hub        *changefeed.Hub
	cleanup    func()
}

func newFunctionsTestEnv(t *testing.T, fake *testutil.FakeDataStorage, configureHandler func(*Server)) *functionsTestEnv {
	t.Helper()
	if fake == nil {
		fake = testutil.NewFakeDataStorage()
	}

	dsConn, cleanupDS := testutil.StartBufServer(t, func(server *grpc.Server) {
		datastoragev1.RegisterDataStorageServiceServer(server, fake)
	})

	validator, err := protocolvalidation.New()
	if err != nil {
		cleanupDS()
		t.Fatalf("validator: %v", err)
	}
	log := logging.New("debug", "atlas-test", "test")
	bundle := datastorageclient.New(datastoragev1.NewDataStorageServiceClient(dsConn))
	hub := changefeed.NewHub()
	funcs := functionpkg.Functions{
		Entity:      functionpkg.NewEntityFunctions(bundle.Entity, log, validator, hub),
		Object:      functionpkg.NewObjectFunctions(bundle.Object, bundle.Idempotency, log, validator, hub),
		Task:        functionpkg.NewTaskFunctions(bundle.Task, bundle.Object, bundle.Entity, bundle.Idempotency, log, validator, hub),
		Observation: functionpkg.NewObservationFunctions(bundle.Observation, log, validator, hub),
	}

	funcConn, cleanupFn := testutil.StartBufServer(t, func(server *grpc.Server) {
		if configureHandler != nil {
			handler := NewServer(funcs, hub, log)
			configureHandler(handler)
			registerFunctionsHandler(server, handler)
			return
		}
		RegisterGRPC(server, funcs, hub, nil)
	})

	env := &functionsTestEnv{
		Client:     functionsv1.NewAtlasFunctionsServiceClient(funcConn),
		Changefeed: functionsv1.NewChangefeedServiceClient(funcConn),
		Fake:       fake,
		Hub:        hub,
		cleanup: func() {
			cleanupFn()
			cleanupDS()
		},
	}
	t.Cleanup(env.cleanup)
	return env
}
