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

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/cmd/flexhub/internal/taskqueue"
	"github.com/nya3jp/flex/internal/flexletpb"
)

type flexletServer struct {
	flexletpb.UnimplementedFlexletServiceServer
	meta  *database.MetaStore
	fs    FS
	queue *taskqueue.TaskQueue
}

func newFlexletServer(meta *database.MetaStore, fs FS) *flexletServer {
	return &flexletServer{
		meta:  meta,
		fs:    fs,
		queue: taskqueue.New(meta),
	}
}

func (s *flexletServer) WaitTask(ctx context.Context, req *flexletpb.WaitTaskRequest) (*flexletpb.WaitTaskResponse, error) {
	job, err := s.queue.WaitPendingJob(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	var tpkgs []*flexletpb.TaskPackage
	for _, jpkg := range job.GetSpec().GetInputs().GetPackages() {
		path := pathForPackage(jpkg.GetId().GetHash())
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

	writeLimit := job.GetSpec().GetLimits().GetTime().AsDuration() + preTaskTime + postTaskTime

	stdoutPath := pathForTask(job.GetId(), stdoutName)
	stdoutURL, err := s.fs.PresignedURLForPut(ctx, stdoutPath, writeLimit)
	if err != nil {
		return nil, err
	}
	stderrPath := pathForTask(job.GetId(), stderrName)
	stderrURL, err := s.fs.PresignedURLForPut(ctx, stderrPath, writeLimit)
	if err != nil {
		return nil, err
	}

	task := &flexletpb.Task{
		Id: job.GetId(),
		Spec: &flexletpb.TaskSpec{
			Command: job.GetSpec().GetCommand(),
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
			Limits: job.GetSpec().GetLimits(),
		},
	}
	return &flexletpb.WaitTaskResponse{Task: task}, nil
}

func (s *flexletServer) UpdateTask(ctx context.Context, req *flexletpb.UpdateTaskRequest) (*flexletpb.UpdateTaskResponse, error) {
	return &flexletpb.UpdateTaskResponse{}, s.meta.UpdateRunningJob(ctx, req.GetId())
}

func (s *flexletServer) ReturnTask(ctx context.Context, req *flexletpb.ReturnTaskRequest) (*flexletpb.ReturnTaskResponse, error) {
	return &flexletpb.ReturnTaskResponse{}, s.meta.ReturnRunningJob(ctx, req.GetId())
}

func (s *flexletServer) FinishTask(ctx context.Context, req *flexletpb.FinishTaskRequest) (*flexletpb.FinishTaskResponse, error) {
	return &flexletpb.FinishTaskResponse{}, s.meta.FinishJob(ctx, req.GetId(), req.GetResult())
}

func (s *flexletServer) UpdateFlexletSpec(ctx context.Context, req *flexletpb.UpdateFlexletSpecRequest) (*flexletpb.UpdateFlexletSpecResponse, error) {
	panic("not implemented")
}
