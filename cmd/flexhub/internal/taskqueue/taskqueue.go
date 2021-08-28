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

package taskqueue

import (
	"context"
	"errors"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/flexletpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TaskQueue struct {
	meta     *database.MetaStore
	waitLock chan struct{}
}

func New(meta *database.MetaStore) *TaskQueue {
	waitLock := make(chan struct{}, 1)
	waitLock <- struct{}{}
	return &TaskQueue{
		meta:     meta,
		waitLock: waitLock,
	}
}

func (q *TaskQueue) WaitTask(ctx context.Context, flexletName string) (*flexletpb.TaskRef, *flex.JobSpec, error) {
	select {
	case <-q.waitLock:
	case <-ctx.Done():
		return nil, nil, fixError(ctx, ctx.Err())
	}
	defer func() { q.waitLock <- struct{}{} }()

	for {
		taskID, spec, err := q.meta.TakeTask(ctx, flexletName)
		if errors.Is(err, database.ErrNoPendingTask) {
			ctxutil.Sleep(ctx, time.Second)
			continue
		}
		if err != nil {
			return nil, nil, fixError(ctx, err)
		}
		return taskID, spec, nil
	}
}

func fixError(ctx context.Context, err error) error {
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	case context.Canceled:
		return status.Error(codes.Canceled, context.Canceled.Error())
	default:
		return err
	}
}
