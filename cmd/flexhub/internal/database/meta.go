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
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/hashutil"
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

func (m *MetaStore) Maintain(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("maintaining: %w", err)
		}
	}()

	// Mark stale flexlets down.
	if _, err := m.db.ExecContext(ctx, `
UPDATE flexlets SET state = 'OFFLINE'
WHERE state = 'ONLINE' AND last_update < TIMESTAMPADD(MINUTE, -1, CURRENT_TIMESTAMP())
`); err != nil {
		return err
	}

	// Release stale jobs.
	if _, err := m.db.ExecContext(ctx, `
UPDATE jobs j INNER JOIN tasks t ON (j.task_uuid = t.uuid)
SET j.state = 'PENDING', j.task_uuid = NULL
WHERE j.state = 'RUNNING' AND t.last_update < TIMESTAMPADD(MINUTE, -1, CURRENT_TIMESTAMP())
`); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) InsertJob(ctx context.Context, spec *flex.JobSpec) (id int64, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("inserting a job: %w", err)
		}
	}()

	priority := spec.GetConstraints().GetPriority()
	req, err := proto.Marshal(spec)
	if err != nil {
		return 0, err
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `INSERT INTO jobs (priority, request) VALUES (?, ?)`, priority, req)
	if err != nil {
		return 0, err
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, label := range spec.GetAnnotations().GetLabels() {
		if _, err := tx.ExecContext(ctx, `INSERT INTO labels (job_id, label) VALUES (?, ?)`, id, label); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

func (m *MetaStore) GetJob(ctx context.Context, id int64) (status *flex.JobStatus, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reading a job: %w", err)
		}
	}()

	rows, err := m.db.QueryContext(ctx, `
SELECT j.id, j.state, j.task_uuid, t.flexlet, j.created, t.started, t.finished, j.request, t.response
FROM jobs j
    LEFT OUTER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE j.id = ?
`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses, err := scanJobStatuses(rows)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("job %d not found", id)
	}

	return statuses[0], nil
}

func (m *MetaStore) ListJobs(ctx context.Context, state flex.JobState, label string, limit int64, beforeID int64) (statuses []*flex.JobStatus, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing jobs: %w", err)
		}
	}()

	query, args := func() (string, []interface{}) {
		if label == "" {
			const query = `
SELECT j.id, j.state, j.task_uuid, t.flexlet, j.created, t.started, t.finished, j.request, t.response
FROM jobs j
    LEFT OUTER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE j.id < ? AND (? OR j.state = ?)
ORDER BY j.id DESC
LIMIT ?
`
			args := []interface{}{
				beforeID,
				state == flex.JobState_UNSPECIFIED,
				formatJobState(state),
				limit,
			}
			return query, args
		}

		const query = `
SELECT j.id, j.state, j.task_uuid, t.flexlet, j.created, t.started, t.finished, j.request, t.response
FROM labels l
	INNER JOIN jobs j ON (l.job_id = j.id)
    LEFT OUTER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE l.label = ? AND l.job_id < ? AND (? OR j.state = ?)
ORDER BY l.job_id DESC
LIMIT ?
`
		args := []interface{}{
			label,
			beforeID,
			state == flex.JobState_UNSPECIFIED,
			formatJobState(state),
			limit,
		}
		return query, args
	}()

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanJobStatuses(rows)
}

func (m *MetaStore) UpdateJobLabels(ctx context.Context, id int64, adds, dels []string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("updating job labels: %w", err)
		}
	}()

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Read the current job spec.
	row := tx.QueryRowContext(ctx, `SELECT request FROM jobs WHERE id = ? FOR UPDATE`, id)
	var req []byte
	if err := row.Scan(&req); err != nil {
		return err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return err
	}

	if spec.Annotations == nil {
		spec.Annotations = &flex.JobAnnotations{}
	}
	olds := spec.Annotations.Labels

	makeSet := func(ss []string) map[string]struct{} {
		m := make(map[string]struct{})
		for _, s := range ss {
			m[s] = struct{}{}
		}
		return m
	}
	addSet := makeSet(adds)
	delSet := makeSet(dels)
	oldSet := makeSet(olds)

	// Update addSet/delSet for consistency.
	for _, old := range olds {
		delete(addSet, old)
	}
	for _, del := range dels {
		if _, ok := oldSet[del]; !ok {
			delete(delSet, del)
		}
	}

	// Compute the new labels.
	var news []string
	for _, old := range olds {
		if _, ok := delSet[old]; !ok {
			news = append(news, old)
		}
	}
	for _, add := range adds {
		if _, ok := addSet[add]; ok {
			news = append(news, add)
		}
	}
	spec.Annotations.Labels = news

	// Save the new spec.
	req, err = proto.Marshal(&spec)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE jobs SET request = ? WHERE id = ?`, req, id); err != nil {
		return err
	}

	// Update label indices.
	for add := range addSet {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO labels (job_id, label) VALUES (?, ?)`, id, add); err != nil {
			return err
		}
	}
	for del := range delSet {
		if _, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE job_id = ? AND label = ?`, id, del); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *MetaStore) TakeTask(ctx context.Context, flexletName string) (ref *flexletpb.TaskRef, jobSpec *flex.JobSpec, err error) {
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
	var jobID int64
	var req []byte
	if err := row.Scan(&jobID, &req); err == sql.ErrNoRows {
		return nil, nil, ErrNoPendingTask
	} else if err != nil {
		return nil, nil, err
	}

	var spec flex.JobSpec
	if err := proto.Unmarshal(req, &spec); err != nil {
		return nil, nil, err
	}

	taskID := uuid.New().String()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO tasks (uuid, flexlet) VALUES (?, ?)
`, taskID, flexletName); err != nil {
		return nil, nil, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET
    state = 'RUNNING',
    task_uuid = ?
WHERE id = ?
`, taskID, jobID); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	ref = &flexletpb.TaskRef{
		TaskId: taskID,
		JobId:  jobID,
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
    last_update = CURRENT_TIMESTAMP()
WHERE uuid = ?
`, ref.GetTaskId()); err != nil {
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
`, nextState, ref.GetJobId(), ref.GetTaskId()); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE tasks
SET
    state = 'FINISHED',
    response = ?,
    finished = CURRENT_TIMESTAMP(),
    last_update = CURRENT_TIMESTAMP()
WHERE uuid = ? AND state = 'RUNNING'
`, response, ref.GetTaskId()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) UpdateTag(ctx context.Context, name, hash string) (err error) {
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
`, name, hash, hash); err != nil {
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

func (m *MetaStore) ListTags(ctx context.Context) (tags []*flex.Tag, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing tags: %w", err)
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
		tags = append(tags, &flex.Tag{
			Hash: hash,
			Name: tag,
		})
	}
	return tags, nil
}

func (m *MetaStore) ListFlexlets(ctx context.Context) (statuses []*flex.FlexletStatus, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("listing flexlets: %w", err)
		}
	}()

	rows, err := m.db.QueryContext(ctx, `SELECT name, state, data FROM flexlets ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statusMap := make(map[string]*flex.FlexletStatus)
	var names []string
	for rows.Next() {
		var name, stateStr string
		var data []byte
		if err := rows.Scan(&name, &stateStr, &data); err != nil {
			return nil, err
		}

		state, err := parseFlexletState(stateStr)
		if err != nil {
			return nil, err
		}

		var spec flex.FlexletSpec
		if err := proto.Unmarshal(data, &spec); err != nil {
			return nil, err
		}

		statusMap[name] = &flex.FlexletStatus{
			Flexlet: &flex.Flexlet{
				Name: name,
				Spec: &spec,
			},
			State: state,
		}
		names = append(names, name)
	}

	rows, err = m.db.QueryContext(ctx, `
SELECT j.id, t.flexlet, j.request
FROM jobs j
    INNER JOIN tasks t ON (j.task_uuid = t.uuid)
WHERE j.state = 'RUNNING'
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var jobID int64
		var flexletName string
		var req []byte
		if err := rows.Scan(&jobID, &flexletName, &req); err != nil {
			return nil, err
		}

		status := statusMap[flexletName]
		if status == nil {
			continue
		}

		var spec flex.JobSpec
		if err := proto.Unmarshal(req, &spec); err != nil {
			return nil, err
		}

		status.CurrentJobs = append(status.CurrentJobs, &flex.Job{
			Id:   jobID,
			Spec: &spec,
		})
	}

	statuses = make([]*flex.FlexletStatus, 0, len(statusMap))
	for _, name := range names {
		statuses = append(statuses, statusMap[name])
	}
	return statuses, nil
}

func (m *MetaStore) UpdateFlexlet(ctx context.Context, status *flex.FlexletStatus) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("updating a flexlet: %w", err)
		}
	}()

	stateStr := formatFlexletState(status.GetState())
	cores := status.GetFlexlet().GetSpec().GetCores()
	data, err := proto.Marshal(status.GetFlexlet().GetSpec())
	if err != nil {
		return err
	}

	if _, err := m.db.ExecContext(ctx, `
INSERT INTO flexlets (name, state, cores, data) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE state = ?, cores = ?, data = ?, last_update = CURRENT_TIMESTAMP()
`, status.GetFlexlet().GetName(), stateStr, cores, data, stateStr, cores, data); err != nil {
		return err
	}
	return nil
}

func (m *MetaStore) GetStats(ctx context.Context) (stats *flex.Stats, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("getting stats: %w", err)
		}
	}()

	row := m.db.QueryRowContext(ctx, `
SELECT
    IFNULL(SUM(IF(state = 'PENDING', 1, 0)), 0),
    IFNULL(SUM(IF(state = 'RUNNING', 1, 0)), 0)
FROM jobs
`)
	var pendingJobs, runningJobs int32
	if err := row.Scan(&pendingJobs, &runningJobs); err != nil {
		return nil, err
	}

	row = m.db.QueryRowContext(ctx, `
SELECT
    IFNULL(SUM(IF(state = 'ONLINE', 1, 0)), 0),
    IFNULL(SUM(IF(state = 'OFFLINE', 1, 0)), 0),
    IFNULL(SUM(IF(state = 'ONLINE' AND cores >= 0, cores, 0)), 0)
FROM flexlets
`)
	var onlineFlexlets, offlineFlexlets, totalFixedCores int32
	if err := row.Scan(&onlineFlexlets, &offlineFlexlets, &totalFixedCores); err != nil {
		return nil, err
	}

	row = m.db.QueryRowContext(ctx, `
SELECT
    IFNULL(SUM(1), 0)
FROM jobs AS j
    INNER JOIN tasks AS t ON (j.task_uuid = t.uuid)
    INNER JOIN flexlets AS f ON (t.flexlet = f.name)
WHERE
    j.state = 'RUNNING' AND
		f.cores >= 0
`)
	var busyFixedCores int32
	if err := row.Scan(&busyFixedCores); err != nil {
		return nil, err
	}

	return &flex.Stats{
		Job: &flex.JobStats{
			PendingJobs: pendingJobs,
			RunningJobs: runningJobs,
		},
		Flexlet: &flex.FlexletStats{
			OnlineFlexlets:  onlineFlexlets,
			OfflineFlexlets: offlineFlexlets,
			BusyCores:       runningJobs,
			IdleCores:       totalFixedCores - busyFixedCores,
		},
	}, nil
}

func scanJobStatuses(rows *sql.Rows) ([]*flex.JobStatus, error) {
	var jobs []*flex.JobStatus
	for rows.Next() {
		var id int64
		var stateStr string
		var taskIDPtr *string
		var flexletNamePtr *string
		var created time.Time
		var started, finished *time.Time
		var req, res []byte
		if err := rows.Scan(&id, &stateStr, &taskIDPtr, &flexletNamePtr, &created, &started, &finished, &req, &res); err != nil {
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

		var taskID string
		if taskIDPtr != nil {
			taskID = *taskIDPtr
		}

		var flexletName string
		if flexletNamePtr != nil {
			flexletName = *flexletNamePtr
		}

		var startedProto, finishedProto *timestamppb.Timestamp
		if started != nil {
			startedProto = timestamppb.New(*started)
		}
		if finished != nil {
			finishedProto = timestamppb.New(*finished)
		}

		jobs = append(jobs, &flex.JobStatus{
			Job: &flex.Job{
				Id:   id,
				Spec: &spec,
			},
			State:       state,
			TaskId:      taskID,
			FlexletName: flexletName,
			Result:      &result,
			Created:     timestamppb.New(created),
			Started:     startedProto,
			Finished:    finishedProto,
		})
	}
	return jobs, nil
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

func parseFlexletState(state string) (flex.FlexletState, error) {
	switch state {
	case "OFFLINE":
		return flex.FlexletState_OFFLINE, nil
	case "ONLINE":
		return flex.FlexletState_ONLINE, nil
	default:
		return flex.FlexletState_OFFLINE, fmt.Errorf("unknown flexlet state %s", state)
	}
}

func formatFlexletState(state flex.FlexletState) string {
	switch state {
	case flex.FlexletState_OFFLINE:
		return "OFFLINE"
	case flex.FlexletState_ONLINE:
		return "ONLINE"
	default:
		return "UNKNOWN"
	}
}
