package service

import (
	"context"
	"errors"

	functionsv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/functions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) SubscribeMutations(req *functionsv1.SubscribeMutationsRequest, stream functionsv1.ChangefeedService_SubscribeMutationsServer) error {
	sub := s.hub.Subscribe(stream.Context())
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				if err := sub.Err(); err != nil {
					if errors.Is(err, context.Canceled) {
						return s.status(stream.Context(), err)
					}
					return status.Error(codes.ResourceExhausted, err.Error())
				}
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		case <-s.hub.Done():
			return s.status(stream.Context(), context.Canceled)
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
