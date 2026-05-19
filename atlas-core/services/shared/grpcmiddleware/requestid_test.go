package grpcmiddleware

import (
	"context"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context {
	return s.ctx
}

func TestRequestIDInterceptors(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		want    string
		wantSet bool
	}{
		{
			name:    "unary generates id",
			ctx:     context.Background(),
			wantSet: true,
		},
		{
			name:    "unary honors metadata",
			ctx:     metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, "client-req-42")),
			want:    "client-req-42",
			wantSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			interceptor := RequestIDUnaryInterceptor()
			_, err := interceptor(tt.ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
				got, _ = logging.RequestIDFromContext(ctx)
				return nil, nil
			})
			if err != nil {
				t.Fatalf("unary interceptor: %v", err)
			}
			if tt.want != "" && got != tt.want {
				t.Fatalf("request id = %q, want %q", got, tt.want)
			}
			if tt.want == "" && got == "" {
				t.Fatal("expected generated request id on context")
			}
		})
	}

	t.Run("stream generates id", func(t *testing.T) {
		var got string
		interceptor := RequestIDStreamInterceptor()
		err := interceptor(nil, &fakeServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{}, func(srv any, stream grpc.ServerStream) error {
			got, _ = logging.RequestIDFromContext(stream.Context())
			return nil
		})
		if err != nil {
			t.Fatalf("stream interceptor: %v", err)
		}
		if got == "" {
			t.Fatal("expected generated request id on stream context")
		}
	})

	t.Run("stream honors metadata", func(t *testing.T) {
		const want = "client-req-42"
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, want))
		var got string
		interceptor := RequestIDStreamInterceptor()
		err := interceptor(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{}, func(srv any, stream grpc.ServerStream) error {
			got, _ = logging.RequestIDFromContext(stream.Context())
			return nil
		})
		if err != nil {
			t.Fatalf("stream interceptor: %v", err)
		}
		if got != want {
			t.Fatalf("request id = %q, want %q", got, want)
		}
	})
}
