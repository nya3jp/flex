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

// AddTask inserts a task to the queue.
func (c *Client) AddTask(ctx context.Context, spec *flexpb.TaskSpec) (id int64, err error) {
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

// GetTask returns a task of a specified ID.
func (c *Client) GetTask(ctx context.Context, id int64) (*flexpb.TaskStatus, error) {
	var stateStr string
	var workerPtr *string
	var req, res []byte
	row := c.db.QueryRowContext(ctx, `SELECT state, worker, request, response FROM tasks WHERE id = ?`, id)
	if err := row.Scan(&stateStr, &workerPtr, &req, &res); err != nil {
		return nil, err
	}

	var spec flexpb.TaskSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, err
	}
	var result flexpb.TaskResult
	if err := proto.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	state, err := parseTaskState(stateStr)
	if err != nil {
		return nil, err
	}
	var worker string
	if workerPtr != nil {
		worker = *workerPtr
	}

	return &flexpb.TaskStatus{
		Task: &flexpb.Task{
			Id:   &flexpb.TaskId{Id: id},
			Spec: &spec,
		},
		State:  state,
		Worker: worker,
		Result: &result,
	}, nil
}

// ListTasks enumerates tasks by descending order of ID.
// limit is the maximum number of tasks returned. beforeID specifies where to
// start enumerating tasks from; only tasks whose ID is smaller than beforeID
// are returned. If you want to enumerate latest tasks, pass math.MaxInt64.
func (c *Client) ListTasks(ctx context.Context, limit, beforeID int64) ([]*flexpb.TaskStatus, error) {
	const query = `SELECT id, state, worker, request, response FROM tasks WHERE id < ? ORDER BY id DESC LIMIT ?`
	rows, err := c.db.QueryContext(ctx, query, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*flexpb.TaskStatus
	for rows.Next() {
		var id int64
		var stateStr string
		var workerPtr *string
		var req, res []byte
		if err := rows.Scan(&id, &stateStr, &workerPtr, &req, &res); err != nil {
			return nil, err
		}

		var spec flexpb.TaskSpec
		if err := proto.Unmarshal(req, &spec); err != nil {
			return nil, err
		}
		var result flexpb.TaskResult
		if err := proto.Unmarshal(res, &result); err != nil {
			return nil, err
		}
		state, err := parseTaskState(stateStr)
		if err != nil {
			return nil, err
		}
		var worker string
		if workerPtr != nil {
			worker = *workerPtr
		}

		tasks = append(tasks, &flexpb.TaskStatus{
			Task: &flexpb.Task{
				Id:   &flexpb.TaskId{Id: id},
				Spec: &spec,
			},
			State:  state,
			Worker: worker,
			Result: &result,
		})
	}
	return tasks, nil
}

func parseTaskState(state string) (flexpb.TaskState, error) {
	switch state {
	case "PENDING":
		return flexpb.TaskState_PENDING, nil
	case "RUNNING":
		return flexpb.TaskState_RUNNING, nil
	case "FINISHED":
		return flexpb.TaskState_FINISHED, nil
	default:
		return flexpb.TaskState_PENDING, fmt.Errorf("unknown task state %s", state)
	}
}
