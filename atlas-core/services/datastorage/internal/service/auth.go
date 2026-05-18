package service

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const authorizationMetadataKey = "authorization"

// InternalAuthUnaryInterceptor rejects datastorage calls without the shared
// internal service credential.
func InternalAuthUnaryInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := requireInternalToken(ctx, token); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// InternalAuthStreamInterceptor rejects datastorage streams without the shared
// internal service credential.
func InternalAuthStreamInterceptor(token string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := requireInternalToken(stream.Context(), token); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

func requireInternalToken(ctx context.Context, token string) error {
	if token == "" {
		return status.Error(codes.Unauthenticated, "datastorage internal token is not configured")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing datastorage internal token")
	}
	for _, value := range md.Get(authorizationMetadataKey) {
		presented, ok := strings.CutPrefix(value, "Bearer ")
		if ok && subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1 {
			return nil
		}
	}
	return status.Error(codes.Unauthenticated, "invalid datastorage internal token")
}
