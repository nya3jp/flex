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
	"github.com/nya3jp/flex/internal/flexlet"
)

func Run(ctx context.Context, cl flexlet.FlexletServiceClient, runner *run.Runner, workers int) error {
	tokens := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		tokens <- struct{}{}
	}

	log.Printf("INFO: Flexlet start")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-tokens:
		case <-ctx.Done():
			return ctx.Err()
		}

		res, err := cl.WaitTask(ctx, &flexlet.WaitTaskRequest{})
		if err != nil {
			tokens <- struct{}{}
			log.Printf("WARNING: WaitTask failed: %v", err)
			ctxutil.Sleep(ctx, 10*time.Second)
			continue
		}

		task := res.GetTask()
		go func() {
			defer func() { tokens <- struct{}{} }()
			stopUpdater := startUpdater(ctx, cl, task.GetId())
			defer stopUpdater()
			log.Printf("INFO: Start task %d: %s", task.GetId().GetIntId(), task.GetSpec().String())
			result := runner.RunTask(ctx, task.GetSpec())
			log.Printf("INFO: End task %d", task.GetId().GetIntId())
			if _, err := cl.FinishTask(ctx, &flexlet.FinishTaskRequest{Id: task.GetId(), Result: result}); err != nil {
				log.Printf("WARNING: FinishTask failed: %v", err)
			}
		}()
	}
}

func startUpdater(ctx context.Context, cl flexlet.FlexletServiceClient, id *flex.JobId) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			_, _ = cl.UpdateTask(ctx, &flexlet.UpdateTaskRequest{Id: id})
			if err := ctxutil.Sleep(ctx, 10*time.Second); err != nil {
				break
			}
		}
	}()
	return cancel
}
