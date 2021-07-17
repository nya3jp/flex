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

	"github.com/nya3jp/flex"
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

func (m *MetaStore) InitTables(ctx context.Context) error {
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

func (m *MetaStore) InsertJob(ctx context.Context, spec *flex.JobSpec) (*flex.JobId, error) {
	priority := spec.GetConstraints().GetPriority()
	req, err := proto.Marshal(spec)
	if err != nil {
		return nil, err
	}

	result, err := m.db.ExecContext(ctx, `INSERT INTO jobs (priority, request) VALUES (?, ?)`, priority, req)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &flex.JobId{IntId: id}, nil
}

func (m *MetaStore) GetJob(ctx context.Context, id *flex.JobId) (*flex.JobStatus, error) {
	var stateStr string
	var flexletPtr *string
	var req, res []byte
	row := m.db.QueryRowContext(ctx, `SELECT state, flexlet, request, response FROM jobs WHERE id = ?`, id.GetIntId())
	if err := row.Scan(&stateStr, &flexletPtr, &req, &res); err != nil {
		return nil, err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, err
	}
	var result flex.JobResult
	if err := proto.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	state, err := parseJobState(stateStr)
	if err != nil {
		return nil, err
	}
	var flexlet *flex.FlexletId
	if flexletPtr != nil {
		flexlet = &flex.FlexletId{Name: *flexletPtr}
	}

	return &flex.JobStatus{
		Id:      id,
		Spec:    &spec,
		State:   state,
		Flexlet: flexlet,
		Result:  &result,
	}, nil
}

func (m *MetaStore) ListJobs(ctx context.Context, limit int64, beforeID *flex.JobId, state flex.JobState) ([]*flex.JobStatus, error) {
	b := beforeID.GetIntId()
	if beforeID == nil {
		b = math.MaxInt64
	}
	query := `SELECT id, state, flexlet, request, response FROM jobs WHERE id < ? ORDER BY id DESC LIMIT ?`
	args := []interface{}{b, limit}
	if state != flex.JobState_UNSPECIFIED {
		query = `SELECT id, state, flexlet, request, response FROM jobs WHERE id < ? AND state = ? ORDER BY id DESC LIMIT ?`
		args = []interface{}{b, formatJobState(state), limit}
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
		var flexletPtr *string
		var req, res []byte
		if err := rows.Scan(&id, &stateStr, &flexletPtr, &req, &res); err != nil {
			return nil, err
		}

		var spec flex.JobSpec
		if err := proto.Unmarshal(req, &spec); err != nil {
			return nil, err
		}
		var result flex.JobResult
		if err := proto.Unmarshal(res, &result); err != nil {
			return nil, err
		}
		state, err := parseJobState(stateStr)
		if err != nil {
			return nil, err
		}
		var flexlet *flex.FlexletId
		if flexletPtr != nil {
			flexlet = &flex.FlexletId{Name: *flexletPtr}
		}

		jobs = append(jobs, &flex.JobStatus{
			Id:      &flex.JobId{IntId: id},
			Spec:    &spec,
			State:   state,
			Flexlet: flexlet,
			Result:  &result,
		})
	}
	return jobs, nil
}

func (m *MetaStore) TakePendingJob(ctx context.Context, flexletID *flex.FlexletId) (*flex.Job, error) {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
SELECT
  id, request
FROM jobs
WHERE
  state = 'PENDING'
ORDER BY priority DESC, id ASC
LIMIT 1`)
	var id int64
	var req []byte
	if err := row.Scan(&id, &req); err == sql.ErrNoRows {
		return nil, ErrNoPendingTask
	} else if err != nil {
		return nil, err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    state = 'RUNNING',
    flexlet = ?,
    started = NOW(),
    last_update = NOW()
WHERE id = ?
`, flexletID.GetName(), id); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &flex.Job{
		Id:   &flex.JobId{IntId: id},
		Spec: &spec,
	}, nil
}

func (m *MetaStore) UpdateRunningJob(ctx context.Context, id *flex.JobId) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    last_update = NOW()
WHERE id = ? AND state = 'RUNNING'
`, id.GetIntId()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) ReturnRunningJob(ctx context.Context, id *flex.JobId) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
	state = 'PENDING',
    flexlet = NULL,
    started = NULL,
    last_update = NOW()
WHERE id = ? AND state = 'RUNNING'
`, id.GetIntId()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) FinishJob(ctx context.Context, id *flex.JobId, result *flex.JobResult) error {
	response, err := proto.Marshal(result)
	if err != nil {
		return err
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    state = 'FINISHED',
    response = ?,
    finished = NOW(),
    last_update = NOW()
WHERE id = ? AND state = 'RUNNING'
`, response, id.GetIntId()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) UpdateTag(ctx context.Context, tag, hash string) error {
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

func (m *MetaStore) LookupTag(ctx context.Context, tag string) (string, error) {
	row := m.db.QueryRowContext(ctx, `SELECT hash FROM tags WHERE tag = ?`, tag)
	var hash string
	if err := row.Scan(&hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (m *MetaStore) ListTags(ctx context.Context) ([]*flex.PackageId, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT tag, hash FROM tags ORDER BY tag ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []*flex.PackageId
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
