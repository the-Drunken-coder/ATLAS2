package datastorageclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const authorizationMetadataKey = "authorization"

// InternalAuthUnaryInterceptor attaches the functions -> datastorage credential
// to every unary RPC.
func InternalAuthUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(withInternalToken(ctx, token), method, req, reply, cc, opts...)
	}
}

// InternalAuthStreamInterceptor attaches the functions -> datastorage credential
// to every streaming RPC.
func InternalAuthStreamInterceptor(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withInternalToken(ctx, token), desc, cc, method, opts...)
	}
}

func withInternalToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authorizationMetadataKey, "Bearer "+token)
}
