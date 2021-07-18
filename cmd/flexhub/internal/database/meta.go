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

package database

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/hashutil"
	"google.golang.org/protobuf/proto"
)

//go:embed schema.mysql.sql
var schemaQueries string

var ErrNoPendingTask = errors.New("no pending task")

type MetaStore struct {
	db *sql.DB
}

func NewMetaStore(db *sql.DB) *MetaStore {
	return &MetaStore{db: db}
}

func (m *MetaStore) InitTables(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("initializing tables: %w", err)
		}
	}()

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, query := range strings.Split(schemaQueries, ";") {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		_, _ = tx.ExecContext(ctx, query)
	}
	return tx.Commit()
}

func (m *MetaStore) InsertJob(ctx context.Context, spec *flex.JobSpec) (id *flex.JobId, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("inserting a job: %w", err)
		}
	}()

	priority := spec.GetConstraints().GetPriority()
	req, err := proto.Marshal(spec)
	if err != nil {
		return nil, err
	}

	result, err := m.db.ExecContext(ctx, `INSERT INTO jobs (priority, request) VALUES (?, ?)`, priority, req)
	if err != nil {
		return nil, err
	}

	intId, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &flex.JobId{IntId: intId}, nil
}

func (m *MetaStore) GetJob(ctx context.Context, id *flex.JobId) (status *flex.JobStatus, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reading a job: %w", err)
		}
	}()

	var stateStr string
	var taskUUIDPtr *string
	var flexletPtr *string
	var req, res []byte
	row := m.db.QueryRowContext(ctx, `
SELECT j.state, j.task_uuid, t.flexlet, j.request, t.response
FROM jobs j
    LEFT OUTER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE j.id = ?
`, id.GetIntId())
	if err := row.Scan(&stateStr, &taskUUIDPtr, &flexletPtr, &req, &res); err != nil {
		return nil, err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, err
	}
	var result flex.TaskResult
	if err := proto.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	state, err := parseJobState(stateStr)
	if err != nil {
		return nil, err
	}

	var taskID *flex.TaskId
	if taskUUIDPtr != nil {
		taskID = &flex.TaskId{Uuid: *taskUUIDPtr}
	}

	var flexlet *flex.FlexletId
	if flexletPtr != nil {
		flexlet = &flex.FlexletId{Name: *flexletPtr}
	}

	return &flex.JobStatus{
		Job: &flex.Job{
			Id:   id,
			Spec: &spec,
		},
		State:     state,
		TaskId:    taskID,
		FlexletId: flexlet,
		Result:    &result,
	}, nil
}

func (m *MetaStore) ListJobs(ctx context.Context, limit int64, beforeID *flex.JobId, state flex.JobState) (statuses []*flex.JobStatus, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing jobs: %w", err)
		}
	}()

	b := beforeID.GetIntId()
	if beforeID == nil {
		b = math.MaxInt64
	}
	const query = `
SELECT j.id, j.state, j.task_uuid, t.flexlet, j.request, t.response
FROM jobs j
    LEFT OUTER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE j.id < ? AND (? OR j.state = ?)
ORDER BY j.id DESC
LIMIT ?
`
	args := []interface{}{
		b,
		state == flex.JobState_UNSPECIFIED,
		formatJobState(state),
		limit,
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*flex.JobStatus
	for rows.Next() {
		var id int64
		var stateStr string
		var taskUUIDPtr *string
		var flexletPtr *string
		var req, res []byte
		if err := rows.Scan(&id, &stateStr, &taskUUIDPtr, &flexletPtr, &req, &res); err != nil {
			return nil, err
		}

		var spec flex.JobSpec
		if err := proto.Unmarshal(req, &spec); err != nil {
			return nil, err
		}
		var result flex.TaskResult
		if err := proto.Unmarshal(res, &result); err != nil {
			return nil, err
		}
		state, err := parseJobState(stateStr)
		if err != nil {
			return nil, err
		}

		var taskID *flex.TaskId
		if taskUUIDPtr != nil {
			taskID = &flex.TaskId{Uuid: *taskUUIDPtr}
		}

		var flexlet *flex.FlexletId
		if flexletPtr != nil {
			flexlet = &flex.FlexletId{Name: *flexletPtr}
		}

		jobs = append(jobs, &flex.JobStatus{
			Job: &flex.Job{
				Id:   &flex.JobId{IntId: id},
				Spec: &spec,
			},
			State:     state,
			TaskId:    taskID,
			FlexletId: flexlet,
			Result:    &result,
		})
	}
	return jobs, nil
}

func (m *MetaStore) TakeTask(ctx context.Context, flexletID *flex.FlexletId) (ref *flexletpb.TaskRef, jobSpec *flex.JobSpec, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("taking a pending task: %w", err)
		}
	}()

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
SELECT
  id, request
FROM jobs
WHERE
  state = 'PENDING'
ORDER BY priority DESC, id ASC
LIMIT 1
FOR UPDATE
`)
	var jobIntId int64
	var req []byte
	if err := row.Scan(&jobIntId, &req); err == sql.ErrNoRows {
		return nil, nil, ErrNoPendingTask
	} else if err != nil {
		return nil, nil, err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, nil, err
	}

	taskUUID := uuid.New().String()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO tasks (uuid, flexlet) VALUES (?, ?)
`, taskUUID, flexletID.GetName()); err != nil {
		return nil, nil, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    state = 'RUNNING',
    task_uuid = ?
WHERE id = ?
`, taskUUID, jobIntId); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	ref = &flexletpb.TaskRef{
		TaskId: &flex.TaskId{Uuid: taskUUID},
		JobId:  &flex.JobId{IntId: jobIntId},
	}
	return ref, &spec, nil
}

func (m *MetaStore) UpdateTask(ctx context.Context, ref *flexletpb.TaskRef) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("updating a running task: %w", err)
		}
	}()

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
    last_update = NOW()
WHERE uuid = ?
`, ref.GetTaskId().GetUuid()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) FinishTask(ctx context.Context, ref *flexletpb.TaskRef, result *flex.TaskResult, needRetry bool) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("finishing a task: %w", err)
		}
	}()

	response, err := proto.Marshal(result)
	if err != nil {
		return err
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	nextState := "FINISHED"
	if needRetry {
		nextState = "PENDING"
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    state = ?
WHERE id = ? AND task_uuid = ? AND state = 'RUNNING'
`, nextState, ref.GetJobId().GetIntId(), ref.GetTaskId().GetUuid()); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
    state = 'FINISHED',
    response = ?,
    finished = NOW(),
    last_update = NOW()
WHERE uuid = ? AND state = 'RUNNING'
`, response, ref.GetTaskId().GetUuid()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) UpdateTag(ctx context.Context, tag, hash string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("updating a tag: %w", err)
		}
	}()

	if !hashutil.IsStdHash(hash) {
		return errors.New("invalid hash")
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := m.db.ExecContext(ctx, `
INSERT INTO tags (tag, hash) VALUES (?, ?)
ON DUPLICATE KEY UPDATE hash = ?
`, tag, hash, hash); err != nil {
		return err
	}

	return tx.Commit()
}

func (m *MetaStore) LookupTag(ctx context.Context, tag string) (hash string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("looking up a tag: %w", err)
		}
	}()
	row := m.db.QueryRowContext(ctx, `SELECT hash FROM tags WHERE tag = ?`, tag)
	if err := row.Scan(&hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (m *MetaStore) ListTags(ctx context.Context) (ids []*flex.PackageId, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("lsting tags: %w", err)
		}
	}()

	rows, err := m.db.QueryContext(ctx, `SELECT tag, hash FROM tags ORDER BY tag ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tag, hash string
		if err := rows.Scan(&tag, &hash); err != nil {
			return nil, err
		}
		ids = append(ids, &flex.PackageId{
			Hash: hash,
			Tag:  tag,
		})
	}
	return ids, nil
}

func parseJobState(state string) (flex.JobState, error) {
	switch state {
	case "PENDING":
		return flex.JobState_PENDING, nil
	case "RUNNING":
		return flex.JobState_RUNNING, nil
	case "FINISHED":
		return flex.JobState_FINISHED, nil
	default:
		return flex.JobState_PENDING, fmt.Errorf("unknown job state %s", state)
	}
}

func formatJobState(state flex.JobState) string {
	switch state {
	case flex.JobState_PENDING:
		return "PENDING"
	case flex.JobState_RUNNING:
		return "RUNNING"
	case flex.JobState_FINISHED:
		return "FINISHED"
	default:
		return "UNKNOWN"
	}
}
