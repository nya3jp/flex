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
	"github.com/nya3jp/flex/internal/concurrent"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/flexletutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RunInPullMode(ctx context.Context, cl flexletpb.FlexletServiceClient, runner *run.Runner, name string, cores int) error {
	limiter := concurrent.NewLimiter(cores)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go flexletutil.RunFletletUpdater(ctx, cl, &flex.Flexlet{Name: name, Spec: &flex.FlexletSpec{Cores: int32(cores)}})

	log.Printf("INFO: Flexlet start")

	for {
		if err := limiter.Take(ctx); err != nil {
			return err
		}

		task, err := waitTaskWithRetry(ctx, cl, name)
		if err != nil {
			limiter.Done()
			return err
		}

		go func() {
			defer limiter.Done()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go flexletutil.RunTaskUpdater(ctx, cl, task.GetRef())

			log.Printf("INFO: Start task %s for job %d", task.GetRef().GetTaskId(), task.GetRef().GetJobId())
			result := runner.RunTask(ctx, task.GetSpec())
			log.Printf("INFO: End task %s for job %d", task.GetRef().GetTaskId(), task.GetRef().GetJobId())
			if _, err := cl.FinishTask(ctx, &flexletpb.FinishTaskRequest{Ref: task.GetRef(), Result: result}); err != nil {
				log.Printf("WARNING: FinishTask failed: %v", err)
			}
		}()
	}
}

func RunInPushMode(ctx context.Context, cl flexletpb.FlexletServiceClient, runner *run.Runner, name string) error {
	task, err := takeTask(ctx, cl, name)
	if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go flexletutil.RunTaskUpdater(ctx, cl, task.GetRef())

	log.Printf("INFO: Start task %s for job %d", task.GetRef().GetTaskId(), task.GetRef().GetJobId())
	result := runner.RunTask(ctx, task.GetSpec())
	log.Printf("INFO: End task %s for job %d", task.GetRef().GetTaskId(), task.GetRef().GetJobId())
	if _, err := cl.FinishTask(ctx, &flexletpb.FinishTaskRequest{Ref: task.GetRef(), Result: result}); err != nil {
		log.Printf("WARNING: FinishTask failed: %v", err)
	}
	return nil
}

func waitTaskWithRetry(ctx context.Context, cl flexletpb.FlexletServiceClient, flexletName string) (*flexletpb.Task, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		task, err := waitTask(ctx, cl, flexletName)
		if err == nil {
			return task, nil
		}
		if s, ok := status.FromError(err); ok && (s.Code() == codes.DeadlineExceeded || s.Code() == codes.Canceled) {
			continue
		}
		log.Printf("WARNING: WaitTask failed: %v", err)
		ctxutil.Sleep(ctx, 10*time.Second)
	}
}

func waitTask(ctx context.Context, cl flexletpb.FlexletServiceClient, flexletName string) (*flexletpb.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	res, err := cl.TakeTask(ctx, &flexletpb.TakeTaskRequest{FlexletName: flexletName, Wait: true})
	if err != nil {
		return nil, err
	}
	return res.GetTask(), nil
}

func takeTask(ctx context.Context, cl flexletpb.FlexletServiceClient, flexletName string) (*flexletpb.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := cl.TakeTask(ctx, &flexletpb.TakeTaskRequest{FlexletName: flexletName, Wait: false})
	if err != nil {
		return nil, err
	}
	return res.GetTask(), nil
}
