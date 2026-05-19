package service

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalAuthUnaryInterceptor(t *testing.T) {
	interceptor := InternalAuthUnaryInterceptor("secret-token")
	handlerCalled := false
	handler := func(context.Context, any) (any, error) {
		handlerCalled = true
		return "ok", nil
	}

	tests := []struct {
		name    string
		ctx     context.Context
		wantErr codes.Code
	}{
		{
			name:    "missing metadata",
			ctx:     context.Background(),
			wantErr: codes.Unauthenticated,
		},
		{
			name: "malformed authorization",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs(authorizationMetadataKey, "Token secret-token"),
			),
			wantErr: codes.Unauthenticated,
		},
		{
			name: "wrong token",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs(authorizationMetadataKey, "Bearer wrong"),
			),
			wantErr: codes.Unauthenticated,
		},
		{
			name: "valid token",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs(authorizationMetadataKey, "Bearer secret-token"),
			),
			wantErr: codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false
			_, err := interceptor(tt.ctx, "request", &grpc.UnaryServerInfo{}, handler)
			if got := status.Code(err); got != tt.wantErr {
				t.Fatalf("expected status %s, got %s (%v)", tt.wantErr, got, err)
			}
			if handlerCalled != (tt.wantErr == codes.OK) {
				t.Fatalf("handler called = %v, want %v", handlerCalled, tt.wantErr == codes.OK)
			}
		})
	}
}

func TestInternalAuthStreamInterceptor(t *testing.T) {
	interceptor := InternalAuthStreamInterceptor("secret-token")
	handlerCalled := false
	handler := func(any, grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(authorizationMetadataKey, "Bearer secret-token"),
	)
	err := interceptor(nil, testServerStream{ctx: ctx}, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected stream handler to be called")
	}

	handlerCalled = false
	err = interceptor(nil, testServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{}, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("expected status %s, got %s (%v)", codes.Unauthenticated, got, err)
	}
	if handlerCalled {
		t.Fatal("stream handler called for unauthenticated request")
	}
}

type testServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s testServerStream) Context() context.Context {
	return s.ctx
}
