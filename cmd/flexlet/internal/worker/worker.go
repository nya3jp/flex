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
	"context"
	"log"

	"github.com/nya3jp/flex/flexpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Options struct {
	Name    string
	RootDir string
}

type Worker struct {
	flexpb.UnimplementedWorkerServer
	opts *Options
	lock chan struct{}
}

var _ flexpb.WorkerServer = &Worker{}

func New(opts *Options) *Worker {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &Worker{
		opts: opts,
		lock: lock,
	}
}

func (w *Worker) RunTask(req *flexpb.RunTaskRequest, srv flexpb.Worker_RunTaskServer) error {
	select {
	case <-w.lock:
	default:
		return status.Error(codes.ResourceExhausted, "another task is running")
	}
	defer func() { w.lock <- struct{}{} }()

	ctx := srv.Context()
	taskID := req.GetTask().GetId().GetId()

	log.Printf("Start task %d", taskID)

	code, err := runTask(ctx, req.GetTask(), w.opts.RootDir, rpcStdout{srv}, rpcStderr{srv})
	var result flexpb.TaskResult
	if err != nil {
		result.Status = &flexpb.TaskResult_Error{Error: err.Error()}
		log.Printf("Failed task %d: %v", taskID, err)
	} else {
		result.Status = &flexpb.TaskResult_ExitCode{ExitCode: int32(code)}
		log.Printf("Succeeded task %d", taskID)
	}
	return srv.Send(&flexpb.RunTaskResponse{Type: &flexpb.RunTaskResponse_Result{Result: &result}})
}

func (w *Worker) GetWorkerInfo(ctx context.Context, req *flexpb.GetWorkerInfoRequest) (*flexpb.GetWorkerInfoResponse, error) {
	return &flexpb.GetWorkerInfoResponse{
		Info: &flexpb.WorkerInfo{
			Name: w.opts.Name,
		},
	}, nil
}

type rpcStdout struct {
	stream flexpb.Worker_RunTaskServer
}

func (r rpcStdout) Write(p []byte) (int, error) {
	err := r.stream.Send(&flexpb.RunTaskResponse{Type: &flexpb.RunTaskResponse_Output{Output: &flexpb.TaskOutput{Stdout: p}}})
	return len(p), err
}

type rpcStderr struct {
	stream flexpb.Worker_RunTaskServer
}

func (r rpcStderr) Write(p []byte) (int, error) {
	err := r.stream.Send(&flexpb.RunTaskResponse{Type: &flexpb.RunTaskResponse_Output{Output: &flexpb.TaskOutput{Stderr: p}}})
	return len(p), err
}
