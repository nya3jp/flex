// Copyright 2020 Google LLC
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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nya3jp/flex/internal/flexlet"
)

const (
	rateUSDJPY       = 110
	cpuCostPerSecond = 0.00002400 * rateUSDJPY     // 1 vCPU
	memCostPerSecond = 0.00000250 * rateUSDJPY * 2 // 2GB
	costPerSecond    = cpuCostPerSecond + memCostPerSecond
	billingUnit      = 100 * time.Millisecond
)

type Job struct {
	Cmd  string `json:"cmd"`
	Pkgs []*Pkg `json:"pkgs"`
}

type Pkg struct {
	URL  string `json:"url"`
	Dest string `json:"dest,omitempty"`
}

type JobResult struct {
	ID       string        `json:"id"`
	Job      Job           `json:"job"`
	Started  time.Time     `json:"started,omitempty"`
	Finished time.Time     `json:"finished,omitempty"`
	Billed   time.Duration `json:"billed"`
	Code     int           `json:"code"`
	Error    string        `json:"error,omitempty"`
	Cost     float64       `json:"cost"`
}

var (
	apiKey             string
	artifactsURLPrefix string
	execURL            string
)

func main() {
	if err := func() error {
		apiKey = os.Getenv("FLEX_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("$FLEX_API_KEY is not set")
		}

		artifactsURLPrefix = os.Getenv("FLEX_ARTIFACTS_URL_PREFIX")
		if artifactsURLPrefix == "" {
			return fmt.Errorf("$FLEX_ARTIFACTS_URL_PREFIX is not set")
		}

		execURL = os.Getenv("FLEX_EXEC_URL")
		if execURL == "" {
			return fmt.Errorf("$FLEX_EXEC_URL is not set")
		}

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			return fmt.Errorf("$PORT must be set correctly: %v", err)
		}
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			return fmt.Errorf("failed to listen: %v", err)
		}

		http.HandleFunc("/run", runHandler)

		fmt.Fprintln(os.Stderr, "Ready")
		return http.Serve(l, nil)
	}(); err != nil {
		panic(fmt.Sprint("ERROR: ", err))
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	handle(w, r, func() error {
		if r.Method != http.MethodPost {
			return errors.New("unsupported method")
		}
		if k := r.Header.Get("X-API-Key"); k != apiKey {
			return errors.New("unauthorized")
		}

		var job Job
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			return err
		}

		res := execJob(&job)
		return json.NewEncoder(w).Encode(res)
	})
}

func handle(w http.ResponseWriter, r *http.Request, f func() error) {
	if err := f(); err != nil {
		msg := fmt.Sprintf("ERROR: %v", err)
		fmt.Fprintln(os.Stderr, msg)
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, msg, http.StatusInternalServerError)
	}
}

func execJob(job *Job) *JobResult {
	var res *JobResult

	if err := func() error {
		id, err := generateID()
		if err != nil {
			return err
		}

		artifactsURL := artifactsURLPrefix + id + "/"

		tres, billed, err := execTask(newTask(job, artifactsURL))
		if err != nil {
			return err
		}

		res = &JobResult{
			ID:       id,
			Job:      *job,
			Started:  tres.Started,
			Finished: tres.Finished,
			Billed:   billed,
			Code:     tres.Code,
			Error:    tres.Error,
			Cost:     (billed.Truncate(billingUnit) + billingUnit).Seconds() * costPerSecond,
		}
		return nil
	}(); err != nil {
		return newErrorJobResult(job, err)
	}
	return res
}

func newErrorJobResult(job *Job, err error) *JobResult {
	return &JobResult{
		Job:   *job,
		Code:  -1,
		Error: err.Error(),
	}
}

func execTask(task *flexlet.Task) (*flexlet.TaskResult, time.Duration, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest(http.MethodPost, execURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-API-Key", apiKey)
	begin := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	billed := time.Since(begin)

	if res.StatusCode != http.StatusOK {
		return nil, billed, fmt.Errorf("flexlet returned %d", res.StatusCode)
	}

	var taskRes flexlet.TaskResult
	if err := json.NewDecoder(res.Body).Decode(&taskRes); err != nil {
		return nil, billed, fmt.Errorf("flexlet returned corrupted response: %v", err)
	}
	return &taskRes, billed, nil
}

func newTask(job *Job, artifactsURL string) *flexlet.Task {
	task := &flexlet.Task{
		Cmd:          job.Cmd,
		Timeout:      30 * time.Second,
		ArtifactsURL: artifactsURL,
	}
	for _, pkg := range job.Pkgs {
		task.Pkgs = append(task.Pkgs, &flexlet.Pkg{
			URL:  pkg.URL,
			Dest: pkg.Dest,
		})
	}
	return task
}

func generateID() (string, error) {
	rnd := make([]byte, 6)
	if _, err := rand.Read(rnd); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", rnd), nil
}
