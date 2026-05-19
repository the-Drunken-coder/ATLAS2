package grpcmiddleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const RequestIDMetadataKey = "x-request-id"

const maxRequestIDLen = 128

var requestIDFallbackCounter uint64

// RequestIDUnaryInterceptor ensures every unary RPC has a request ID on context.
func RequestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(withRequestID(ctx), req)
	}
}

// RequestIDStreamInterceptor ensures every streaming RPC has a request ID on context.
func RequestIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &requestIDServerStream{ServerStream: stream, ctx: withRequestID(stream.Context())})
	}
}

type requestIDServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *requestIDServerStream) Context() context.Context {
	return s.ctx
}

func withRequestID(ctx context.Context) context.Context {
	if id, ok := logging.RequestIDFromContext(ctx); ok && id != "" {
		return ctx
	}
	return logging.ContextWithRequestID(ctx, resolveRequestID(ctx))
}

func resolveRequestID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(RequestIDMetadataKey); len(values) > 0 {
			for _, value := range values {
				if id := sanitizeRequestID(value); id != "" {
					return id
				}
			}
		}
	}
	return newRequestID()
}

func sanitizeRequestID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" || len(id) > maxRequestIDLen {
		return ""
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return ""
		}
	}
	return id
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := atomic.AddUint64(&requestIDFallbackCounter, 1)
		return "fallback-" + strconv.FormatInt(time.Now().UnixNano(), 16) + "-" + strconv.FormatUint(n, 16)
	}
	return hex.EncodeToString(b[:])
}
