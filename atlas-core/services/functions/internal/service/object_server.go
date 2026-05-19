package service

import (
	"context"

	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) CreateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(defaultObjectRequestTimestamps(req.GetObject()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	var opts []functionpkg.IdempotencyOption
	if req.IdempotencyKey != nil && req.GetIdempotencyKey() != "" {
		opts = append(opts, functionpkg.WithIdempotencyKey(req.GetIdempotencyKey()))
	}
	if err := s.funcs.Object.CreateObject(ctx, object, opts...); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) GetObject(ctx context.Context, req *sharedv1.GetObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := s.funcs.Object.GetObject(ctx, req.GetObjectId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) ListObjects(ctx context.Context, req *sharedv1.ListObjectsRequest) (*sharedv1.ListObjectsResponse, error) {
	filters, err := pbconv.ObjectFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	result, err := s.funcs.Object.ListObjects(ctx, store.ObjectListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, s.status(ctx, err)
	}
	resp := &sharedv1.ListObjectsResponse{NextPageToken: result.NextPageToken}
	for i := range result.Objects {
		resp.Objects = append(resp.Objects, pbconv.ObjectToProto(&result.Objects[i]))
	}
	return resp, nil
}
func (s *Server) UpdateObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(req.GetObject())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Object.UpdateObject(ctx, object); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) DeleteObject(ctx context.Context, req *sharedv1.DeleteObjectRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Object.DeleteObject(ctx, req.GetObjectId()); err != nil {
		return nil, s.status(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertObject(ctx context.Context, req *sharedv1.ObjectRequest) (*sharedv1.ObjectResponse, error) {
	object, err := pbconv.ObjectFromProto(defaultObjectRequestTimestamps(req.GetObject()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Object.UpsertObject(ctx, object); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectResponse{Object: pbconv.ObjectToProto(object)}, nil
}
func (s *Server) GetObjectManifest(ctx context.Context, req *sharedv1.GetObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := s.funcs.Object.GetObjectManifest(ctx, req.GetObjectId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *Server) UpdateObjectManifest(ctx context.Context, req *sharedv1.UpdateObjectManifestRequest) (*sharedv1.ObjectManifestResponse, error) {
	manifest, err := pbconv.ManifestFromProto(req.GetManifest())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Object.UpdateObjectManifest(ctx, req.GetObjectId(), manifest); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectManifestResponse{Manifest: pbconv.ManifestToProto(manifest), ManifestCurrent: true}, nil
}
func (s *Server) DeleteObjectFile(ctx context.Context, req *sharedv1.ReadFileRequest) (*sharedv1.ObjectManifestResponse, error) {
	result, err := s.funcs.Object.DeleteFile(ctx, req.GetObjectId(), req.GetFilename())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ObjectManifestResponse{
		Manifest:          pbconv.ManifestToProto(result.Manifest),
		ManifestCurrent:   result.ManifestCurrent,
		ManifestSyncError: result.ManifestSyncError,
	}, nil
}
func (s *Server) ListObjectFiles(ctx context.Context, req *sharedv1.ListObjectFilesRequest) (*sharedv1.ListObjectFilesResponse, error) {
	files, err := s.funcs.Object.ListFiles(ctx, req.GetObjectId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.ListObjectFilesResponse{Filenames: files}, nil
}
