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

package flexlet

import (
	"context"
	"log"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/flexletpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Run(ctx context.Context, cl flexletpb.FlexletServiceClient, runner *run.Runner, flexletID *flex.FlexletId, cores int) error {
	tokens := make(chan struct{}, cores)
	for i := 0; i < cores; i++ {
		tokens <- struct{}{}
	}

	log.Printf("INFO: Flexlet start")

	for {
		select {
		case <-tokens:
		case <-ctx.Done():
			return ctx.Err()
		}

		task, err := waitTaskWithRetry(ctx, cl, flexletID)
		if err != nil {
			return err
		}

		go func() {
			defer func() { tokens <- struct{}{} }()
			stopUpdater := startUpdater(ctx, cl, task.GetRef())
			defer stopUpdater()
			log.Printf("INFO: Start task %s for job %d: %s", task.GetRef().GetTaskId().GetUuid(), task.GetRef().GetJobId().GetIntId(), task.GetSpec().String())
			result := runner.RunTask(ctx, task.GetSpec())
			log.Printf("INFO: End task %s for job %d", task.GetRef().GetTaskId().GetUuid(), task.GetRef().GetJobId().GetIntId())
			if _, err := cl.FinishTask(ctx, &flexletpb.FinishTaskRequest{Ref: task.GetRef(), Result: result}); err != nil {
				log.Printf("WARNING: FinishTask failed: %v", err)
			}
		}()
	}
}

func waitTaskWithRetry(ctx context.Context, cl flexletpb.FlexletServiceClient, flexletID *flex.FlexletId) (*flexletpb.Task, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		task, err := waitTask(ctx, cl, flexletID)
		if err == nil {
			return task, nil
		}
		if s, ok := status.FromError(err); ok && s.Code() == codes.DeadlineExceeded {
			continue
		}
		log.Printf("WARNING: WaitTask failed: %v", err)
		ctxutil.Sleep(ctx, 10*time.Second)
	}
}

func waitTask(ctx context.Context, cl flexletpb.FlexletServiceClient, flexletID *flex.FlexletId) (*flexletpb.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	res, err := cl.WaitTask(ctx, &flexletpb.WaitTaskRequest{FlexletId: flexletID})
	if err != nil {
		return nil, err
	}
	return res.GetTask(), nil
}

func startUpdater(ctx context.Context, cl flexletpb.FlexletServiceClient, ref *flexletpb.TaskRef) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			_, _ = cl.UpdateTask(ctx, &flexletpb.UpdateTaskRequest{Ref: ref})
			if err := ctxutil.Sleep(ctx, 10*time.Second); err != nil {
				break
			}
		}
	}()
	return cancel
}
