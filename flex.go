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

package flex

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nya3jp/flex/flexpb"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	db *sql.DB
}

func NewClient(db *sql.DB) *Client {
	return &Client{db: db}
}

func (c *Client) AddTask(ctx context.Context, spec *flexpb.TaskSpec) (int64, error) {
	prio := spec.GetConstraints().GetPriority()
	req, err := proto.Marshal(spec)
	if err != nil {
		return 0, err
	}

	result, err := c.db.ExecContext(ctx, `INSERT INTO tasks (priority, request) VALUES (?, ?)`, prio, req)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (c *Client) GetTaskStatus(ctx context.Context, id int64) (*flexpb.TaskStatus, error) {
	var stateStr, worker string
	var req []byte
	row := c.db.QueryRowContext(ctx, `SELECT state, worker, request FROM tasks WHERE id = ?`, id)
	if err := row.Scan(&stateStr, &worker, &req); err != nil {
		return nil, err
	}

	var spec flexpb.TaskSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, err
	}

	var state flexpb.TaskState
	switch stateStr {
	case "PENDING":
		state = flexpb.TaskState_PENDING
	case "RUNNING":
		state = flexpb.TaskState_RUNNING
	case "FINISHED":
		state = flexpb.TaskState_FINISHED
	default:
		return nil, fmt.Errorf("unknown task state %s", stateStr)
	}

	return &flexpb.TaskStatus{
		Task: &flexpb.Task{
			Id:   &flexpb.TaskId{Id: id},
			Spec: &spec,
		},
		State:  state,
		Worker: worker,
	}, nil
}
