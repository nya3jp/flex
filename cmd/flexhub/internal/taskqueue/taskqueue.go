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
	"database/sql"
	"errors"
	"fmt"

	"github.com/nya3jp/flex/flexpb"
	"google.golang.org/protobuf/proto"
)

type TaskQueue struct {
	db *sql.DB
}

func New(db *sql.DB) *TaskQueue {
	return &TaskQueue{db: db}
}

var ErrNoPendingTask = errors.New("no pending task")

func (d *TaskQueue) Take(ctx context.Context, worker string) (*flexpb.Task, error) {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("taking task: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
SELECT
  id, request
FROM tasks
WHERE
  state = 'PENDING'
ORDER BY priority DESC, id ASC
LIMIT 1`)
	var id int64
	var req []byte
	if err := row.Scan(&id, &req); err == sql.ErrNoRows {
		return nil, ErrNoPendingTask
	} else if err != nil {
		return nil, fmt.Errorf("taking task: %w", err)
	}

	var spec flexpb.TaskSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, fmt.Errorf("taking task: unmarshaling task spec: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
    state = 'RUNNING',
    worker = ?,
    started = NOW()
WHERE id = ?
`, worker, id); err != nil {
		return nil, fmt.Errorf("taking task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("taking task: %w", err)
	}

	return &flexpb.Task{
		Id:   &flexpb.TaskId{Id: id},
		Spec: &spec,
	}, nil
}

func (d *TaskQueue) Reset(ctx context.Context, id int64) error {
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("resetting task: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
	state = 'PENDING',
    worker = NULL,
    started = NULL
WHERE id = ?
`, id); err != nil {
		return fmt.Errorf("resetting task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("resetting task: %w", err)
	}
	return nil
}

func (d *TaskQueue) Finish(ctx context.Context, id int64, result *flexpb.TaskResult) error {
	response, err := proto.Marshal(result)
	if err != nil {
		return fmt.Errorf("finishing task: %w", err)
	}

	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("finishing task: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
    state = 'FINISHED',
    finished = NOW(),
    response = ?
WHERE id = ?
`, response, id); err != nil {
		return fmt.Errorf("finishing task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("finishing task: %w", err)
	}
	return nil
}
