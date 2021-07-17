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

package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/filecache"
	"github.com/nya3jp/flex/internal/flexlet"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Runner struct {
	tasksDir string
	cache    *filecache.Manager
}

func New(storeDir string) (*Runner, error) {
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return nil, err
	}

	tasksDir := filepath.Join(storeDir, "tasks")
	cacheDir := filepath.Join(storeDir, "cache")
	for _, dir := range []string{tasksDir, cacheDir} {
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	cache := filecache.NewManager(cacheDir)
	return &Runner{
		tasksDir: tasksDir,
		cache:    cache,
	}, nil
}

func (r *Runner) RunTask(ctx context.Context, spec *flexlet.TaskSpec) *flex.JobResult {
	code, dur, err := runTask(ctx, spec, r.tasksDir, r.cache)
	result := &flex.JobResult{
		Time: durationpb.New(dur),
	}
	if err == nil {
		result.Status = &flex.JobResult_ExitCode{ExitCode: int32(code)}
	} else {
		result.Status = &flex.JobResult_Error{Error: err.Error()}
	}
	return result
}

func runTask(ctx context.Context, spec *flexlet.TaskSpec, tasksDir string, cache *filecache.Manager) (code int, dur time.Duration, err error) {
	taskDir, err := ioutil.TempDir(tasksDir, "")
	if err != nil {
		return -1, 0, fmt.Errorf("failed to create a task directory: %w", err)
	}
	defer os.RemoveAll(taskDir)

	execDir := filepath.Join(taskDir, "exec")
	outDir := filepath.Join(taskDir, "out")
	for _, dir := range []string{execDir, outDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return -1, 0, err
		}
	}

	if err := prepareInputs(ctx, execDir, spec.GetInputs(), cache); err != nil {
		return -1, 0, fmt.Errorf("failed to prepare task: %w", err)
	}

	stdout, err := os.Create(filepath.Join(taskDir, "stdout.txt"))
	if err != nil {
		return -1, 0, err
	}
	defer stdout.Close()

	stderr, err := os.Create(filepath.Join(taskDir, "stderr.txt"))
	if err != nil {
		return -1, 0, err
	}
	defer stderr.Close()

	start := time.Now()
	code, execErr := execCmd(ctx, outDir, execDir, spec.GetCommand(), stdout, stderr, spec.GetLimits())
	dur = time.Since(start)

	if err := uploadOutputs(ctx, spec.GetOutputs(), stdout, stderr); err != nil {
		log.Printf("WARNING: Uploading outputs failed: %v", err)
	}

	if execErr != nil {
		return -1, dur, fmt.Errorf("task execution failed: %w", execErr)
	}
	return code, dur, nil
}

func prepareInputs(ctx context.Context, execDir string, inputs *flexlet.TaskInputs, cache *filecache.Manager) error {
	for _, pkg := range inputs.GetPackages() {
		if err := preparePackage(ctx, execDir, pkg, cache); err != nil {
			return err
		}
	}
	return nil
}

func preparePackage(ctx context.Context, execDir string, pkg *flexlet.TaskPackage, cache *filecache.Manager) error {
	extractDir := execDir
	if dir := pkg.GetInstallDir(); dir != "" {
		extractDir = filepath.Join(execDir, dir)
		if err := os.MkdirAll(extractDir, 0700); err != nil {
			return err
		}
	}

	f, err := openLocation(ctx, pkg.GetLocation(), cache)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.CommandContext(ctx, "tar", "xz")
	cmd.Dir = extractDir
	cmd.Stdin = f
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func uploadOutputs(ctx context.Context, outputs *flexlet.TaskOutputs, stdout, stderr *os.File) error {
	var firstErr error
	if err := putLocation(ctx, outputs.GetStdout(), stdout); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := putLocation(ctx, outputs.GetStderr(), stderr); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func openLocation(ctx context.Context, loc *flex.FileLocation, cache *filecache.Manager) (io.ReadCloser, error) {
	return cache.Open(loc.GetCanonicalUrl(), func(w io.Writer) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.GetPresignedUrl(), nil)
		if err != nil {
			return err
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		_, err = io.Copy(w, res.Body)
		return err
	})
}

func putLocation(ctx context.Context, loc *flex.FileLocation, f io.ReadSeeker) error {
	if loc == nil {
		return nil
	}

	parsed, err := url.Parse(loc.GetCanonicalUrl())
	if err != nil {
		return err
	}

	switch parsed.Scheme {
	case "file":
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}

		w, err := os.Create(parsed.Path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := w.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	case "s3":
		size, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, loc.GetPresignedUrl(), struct{ io.ReadSeeker }{f})
		if err != nil {
			return err
		}
		req.ContentLength = size
		if size == 0 {
			// Avoid 501 Not Implemented on size=0.
			req.Body = nil
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New(res.Status)
		}
		return nil
	default:
		return fmt.Errorf("unknown scheme: %s", parsed.Scheme)
	}
}

func execCmd(ctx context.Context, outDir, execDir string, cmd *flex.JobCommand, stdout, stderr io.Writer, limits *flex.JobLimits) (code int, err error) {
	origCtx := ctx
	timeLimit := limits.GetTime().AsDuration()
	ctx, cancel := context.WithTimeout(ctx, timeLimit)
	defer cancel()

	c := exec.CommandContext(ctx, "sh", "-c", cmd.GetShell())
	c.Dir = execDir
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = append(os.Environ(), "OUT_DIR="+outDir)
	if err := c.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded && origCtx.Err() == nil {
			return -1, fmt.Errorf("timeout reached (%v)", timeLimit)
		}
		if errExit, ok := err.(*exec.ExitError); ok {
			if status, ok := errExit.Sys().(syscall.WaitStatus); ok {
				if status.Exited() {
					return status.ExitStatus(), nil
				}
				if status.Signaled() {
					return 0, fmt.Errorf("signal: %s", unix.SignalName(status.Signal()))
				}
			}
		}
		return 0, err
	}
	return 0, nil
}
