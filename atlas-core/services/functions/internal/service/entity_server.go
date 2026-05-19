package service

import (
	"context"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) CreateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(defaultEntityRequestTimestamps(req.GetEntity()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Entity.CreateEntity(ctx, entity); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) GetEntity(ctx context.Context, req *sharedv1.GetEntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := s.funcs.Entity.GetEntity(ctx, req.GetEntityId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) ListEntities(ctx context.Context, req *sharedv1.ListEntitiesRequest) (*sharedv1.ListEntitiesResponse, error) {
	filters, err := pbconv.EntityFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	result, err := s.funcs.Entity.ListEntities(ctx, store.EntityListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, s.status(ctx, err)
	}
	resp := &sharedv1.ListEntitiesResponse{NextPageToken: result.NextPageToken}
	for i := range result.Entities {
		resp.Entities = append(resp.Entities, pbconv.EntityToProto(&result.Entities[i]))
	}
	return resp, nil
}
func (s *Server) UpdateEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(req.GetEntity())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Entity.UpdateEntity(ctx, entity); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
func (s *Server) DeleteEntity(ctx context.Context, req *sharedv1.DeleteEntityRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Entity.DeleteEntity(ctx, req.GetEntityId()); err != nil {
		return nil, s.status(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertEntity(ctx context.Context, req *sharedv1.EntityRequest) (*sharedv1.EntityResponse, error) {
	entity, err := pbconv.EntityFromProto(defaultEntityRequestTimestamps(req.GetEntity()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Entity.UpsertEntity(ctx, entity); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.EntityResponse{Entity: pbconv.EntityToProto(entity)}, nil
}
