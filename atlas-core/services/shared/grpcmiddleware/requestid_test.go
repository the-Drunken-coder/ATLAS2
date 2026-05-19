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

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context {
	return s.ctx
}

func TestRequestIDStreamInterceptorGeneratesID(t *testing.T) {
	var got string
	interceptor := RequestIDStreamInterceptor()
	err := interceptor(nil, &fakeServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{}, func(srv any, stream grpc.ServerStream) error {
		got, _ = logging.RequestIDFromContext(stream.Context())
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if got == "" {
		t.Fatal("expected generated request id on stream context")
	}
}

func TestRequestIDStreamInterceptorHonorsIncomingMetadata(t *testing.T) {
	const want = "client-req-42"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, want))
	var got string
	interceptor := RequestIDStreamInterceptor()
	err := interceptor(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{}, func(srv any, stream grpc.ServerStream) error {
		got, _ = logging.RequestIDFromContext(stream.Context())
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if got != want {
		t.Fatalf("request id = %q, want %q", got, want)
	}
}
