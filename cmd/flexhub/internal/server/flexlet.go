// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/cmd/flexhub/internal/waitqueue"
	"github.com/nya3jp/flex/internal/flexletpb"
)

type flexletServer struct {
	flexletpb.UnimplementedFlexletServiceServer
	meta  *database.MetaStore
	fs    FS
	queue *waitqueue.WaitQueue
}

func newFlexletServer(meta *database.MetaStore, fs FS) *flexletServer {
	return &flexletServer{
		meta:  meta,
		fs:    fs,
		queue: waitqueue.New(meta),
	}
}

func (s *flexletServer) TakeTask(ctx context.Context, req *flexletpb.TakeTaskRequest) (*flexletpb.TakeTaskResponse, error) {
	ref, jobSpec, err := func() (*flexletpb.TaskRef, *flex.JobSpec, error) {
		if !req.GetWait() {
			return s.meta.TakeTask(ctx, req.GetFlexletName())
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		return s.queue.WaitTask(waitCtx, req.GetFlexletName())
	}()
	if errors.Is(err, database.ErrNoPendingTask) {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, err
	}

	var tpkgs []*flexletpb.TaskPackage
	for _, jpkg := range jobSpec.GetInputs().GetPackages() {
		path := pathForPackage(jpkg.GetHash())
		url, err := s.fs.PresignedURLForGet(ctx, path, preTaskTime)
		if err != nil {
			return nil, err
		}
		tpkgs = append(tpkgs, &flexletpb.TaskPackage{
			Location: &flex.FileLocation{
				CanonicalUrl: s.fs.CanonicalURL(path),
				PresignedUrl: url,
			},
			InstallDir: jpkg.GetInstallDir(),
		})
	}

	writeLimit := jobSpec.GetLimits().GetTime().AsDuration() + preTaskTime + postTaskTime

	stdoutPath := pathForTask(ref.GetTaskId(), stdoutName)
	stdoutURL, err := s.fs.PresignedURLForPut(ctx, stdoutPath, writeLimit)
	if err != nil {
		return nil, err
	}
	stderrPath := pathForTask(ref.GetTaskId(), stderrName)
	stderrURL, err := s.fs.PresignedURLForPut(ctx, stderrPath, writeLimit)
	if err != nil {
		return nil, err
	}

	task := &flexletpb.Task{
		Ref: ref,
		Spec: &flexletpb.TaskSpec{
			Command: jobSpec.GetCommand(),
			Inputs:  &flexletpb.TaskInputs{Packages: tpkgs},
			Outputs: &flexletpb.TaskOutputs{
				Stdout: &flex.FileLocation{
					CanonicalUrl: s.fs.CanonicalURL(stdoutPath),
					PresignedUrl: stdoutURL,
				},
				Stderr: &flex.FileLocation{
					CanonicalUrl: s.fs.CanonicalURL(stderrPath),
					PresignedUrl: stderrURL,
				},
			},
			Limits: jobSpec.GetLimits(),
		},
	}
	return &flexletpb.TakeTaskResponse{Task: task}, nil
}

func (s *flexletServer) UpdateTask(ctx context.Context, req *flexletpb.UpdateTaskRequest) (*flexletpb.UpdateTaskResponse, error) {
	return &flexletpb.UpdateTaskResponse{}, s.meta.UpdateTask(ctx, req.GetRef())
}

func (s *flexletServer) FinishTask(ctx context.Context, req *flexletpb.FinishTaskRequest) (*flexletpb.FinishTaskResponse, error) {
	return &flexletpb.FinishTaskResponse{}, s.meta.FinishTask(ctx, req.GetRef(), req.GetResult(), req.GetNeedRetry())
}

func (s *flexletServer) UpdateFlexlet(ctx context.Context, req *flexletpb.UpdateFlexletRequest) (*flexletpb.UpdateFlexletResponse, error) {
	return &flexletpb.UpdateFlexletResponse{}, s.meta.UpdateFlexlet(ctx, req.GetStatus())
}
