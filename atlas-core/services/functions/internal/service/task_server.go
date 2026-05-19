package service

import (
	"context"

	functionpkg "github.com/anomalyco/atlas-core/services/functions/internal/function"
	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
	"github.com/anomalyco/atlas-core/services/shared/pbconv"
	"github.com/anomalyco/atlas-core/services/shared/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) CreateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(defaultTaskRequestTimestamps(req.GetTask()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	var opts []functionpkg.IdempotencyOption
	if req.IdempotencyKey != nil && req.GetIdempotencyKey() != "" {
		opts = append(opts, functionpkg.WithIdempotencyKey(req.GetIdempotencyKey()))
	}
	if err := s.funcs.Task.CreateTask(ctx, task, opts...); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) GetTask(ctx context.Context, req *sharedv1.GetTaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := s.funcs.Task.GetTask(ctx, req.GetTaskId())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) ListTasks(ctx context.Context, req *sharedv1.ListTasksRequest) (*sharedv1.ListTasksResponse, error) {
	filters, err := pbconv.TaskFiltersFromProto(req.GetFilter())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	result, err := s.funcs.Task.ListTasks(ctx, store.TaskListParams{
		Filters:   filters,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, s.status(ctx, err)
	}
	resp := &sharedv1.ListTasksResponse{NextPageToken: result.NextPageToken}
	for i := range result.Tasks {
		resp.Tasks = append(resp.Tasks, pbconv.TaskToProto(&result.Tasks[i]))
	}
	return resp, nil
}
func (s *Server) UpdateTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(req.GetTask())
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Task.UpdateTask(ctx, task); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
func (s *Server) DeleteTask(ctx context.Context, req *sharedv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.funcs.Task.DeleteTask(ctx, req.GetTaskId()); err != nil {
		return nil, s.status(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
func (s *Server) UpsertTask(ctx context.Context, req *sharedv1.TaskRequest) (*sharedv1.TaskResponse, error) {
	task, err := pbconv.TaskFromProto(defaultTaskRequestTimestamps(req.GetTask()))
	if err != nil {
		return nil, s.status(ctx, err)
	}
	if err := s.funcs.Task.UpsertTask(ctx, task); err != nil {
		return nil, s.status(ctx, err)
	}
	return &sharedv1.TaskResponse{Task: pbconv.TaskToProto(task)}, nil
}
