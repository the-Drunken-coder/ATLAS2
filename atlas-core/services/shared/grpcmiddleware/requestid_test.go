package grpcmiddleware

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDUnaryInterceptorGeneratesID(t *testing.T) {
	var got string
	interceptor := RequestIDUnaryInterceptor()
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		got, _ = logging.RequestIDFromContext(ctx)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if got == "" {
		t.Fatal("expected generated request id on context")
	}
}

func TestRequestIDUnaryInterceptorHonorsIncomingMetadata(t *testing.T) {
	const want = "client-req-42"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, want))
	var got string
	interceptor := RequestIDUnaryInterceptor()
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		got, _ = logging.RequestIDFromContext(ctx)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if got != want {
		t.Fatalf("request id = %q, want %q", got, want)
	}
}
