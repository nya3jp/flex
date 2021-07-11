// Copyright 2020 Google LLC
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

package worker

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nya3jp/flex"
)

type Options struct {
	RootDir string
}

type Worker struct {
	flex.UnimplementedWorkerServer
	opts *Options
	lock chan struct{}
}

var _ flex.WorkerServer = &Worker{}

func New(opts *Options) *Worker {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &Worker{
		opts: opts,
		lock: lock,
	}
}

func (w *Worker) RunTask(req *flex.RunTaskRequest, srv flex.Worker_RunTaskServer) error {
	select {
	case <-w.lock:
	default:
		return status.Error(codes.ResourceExhausted, "another task is running")
	}
	defer func() { w.lock <- struct{}{} }()

	ctx := srv.Context()

	code, err := runTask(ctx, req.GetTask(), w.opts.RootDir, rpcStdout{srv}, rpcStderr{srv})
	var result flex.TaskResult
	if err != nil {
		result.Status = &flex.TaskResult_Error{Error: err.Error()}
	} else {
		result.Status = &flex.TaskResult_ExitCode{ExitCode: int32(code)}
	}
	return srv.Send(&flex.RunTaskResponse{Type: &flex.RunTaskResponse_Result{Result: &result}})
}

type rpcStdout struct {
	stream flex.Worker_RunTaskServer
}

func (r rpcStdout) Write(p []byte) (int, error) {
	err := r.stream.Send(&flex.RunTaskResponse{Type: &flex.RunTaskResponse_Output{Output: &flex.TaskOutput{Stdout: p}}})
	return len(p), err
}

type rpcStderr struct {
	stream flex.Worker_RunTaskServer
}

func (r rpcStderr) Write(p []byte) (int, error) {
	err := r.stream.Send(&flex.RunTaskResponse{Type: &flex.RunTaskResponse_Output{Output: &flex.TaskOutput{Stderr: p}}})
	return len(p), err
}
