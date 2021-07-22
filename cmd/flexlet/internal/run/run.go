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
	"github.com/nya3jp/flex/internal/flexletpb"
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

func (r *Runner) RunTask(ctx context.Context, spec *flexletpb.TaskSpec) *flex.TaskResult {
	taskDir, err := ioutil.TempDir(r.tasksDir, "")
	if err != nil {
		return &flex.TaskResult{
			ExitCode: -1,
			Message:  fmt.Sprintf("failed to create a task directory: %v", err),
		}
	}
	defer os.RemoveAll(taskDir)

	execDir := filepath.Join(taskDir, "exec")
	outDir := filepath.Join(taskDir, "out")
	for _, dir := range []string{execDir, outDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return &flex.TaskResult{
				ExitCode: -1,
				Message:  fmt.Sprintf("failed to prepare a task: %v", err),
			}
		}
	}

	if err := prepareInputs(ctx, execDir, spec.GetInputs(), r.cache); err != nil {
		return &flex.TaskResult{
			ExitCode: -1,
			Message:  fmt.Sprintf("failed to prepare a task: %v", err),
		}
	}

	stdout, err := os.Create(filepath.Join(taskDir, "stdout.txt"))
	if err != nil {
		return &flex.TaskResult{
			ExitCode: -1,
			Message:  fmt.Sprintf("failed to prepare task stdout: %v", err),
		}
	}
	defer stdout.Close()

	stderr, err := os.Create(filepath.Join(taskDir, "stderr.txt"))
	if err != nil {
		return &flex.TaskResult{
			ExitCode: -1,
			Message:  fmt.Sprintf("failed to prepare task stderr: %v", err),
		}
	}
	defer stderr.Close()

	start := time.Now()
	code, execErr := execCmd(ctx, outDir, execDir, spec.GetCommand(), stdout, stderr, spec.GetLimits())
	dur := time.Since(start)

	if err := uploadOutputs(ctx, spec.GetOutputs(), stdout, stderr); err != nil {
		log.Printf("WARNING: Uploading outputs failed: %v", err)
	}

	result := &flex.TaskResult{
		ExitCode: int32(code),
		Time:     durationpb.New(dur),
	}
	if execErr != nil {
		result.Message = execErr.Error()
	} else {
		result.Message = "success"
	}
	return result
}

func prepareInputs(ctx context.Context, execDir string, inputs *flexletpb.TaskInputs, cache *filecache.Manager) error {
	for _, pkg := range inputs.GetPackages() {
		if err := preparePackage(ctx, execDir, pkg, cache); err != nil {
			return err
		}
	}
	return nil
}

func preparePackage(ctx context.Context, execDir string, pkg *flexletpb.TaskPackage, cache *filecache.Manager) error {
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

func uploadOutputs(ctx context.Context, outputs *flexletpb.TaskOutputs, stdout, stderr *os.File) error {
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
	case "gs", "s3", "http":
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
		if res.StatusCode/100 != 2 {
			return errors.New(res.Status)
		}
		return nil
	default:
		return fmt.Errorf("unknown scheme: %s", parsed.Scheme)
	}
}

func execCmd(ctx context.Context, outDir, execDir string, cmd *flex.JobCommand, stdout, stderr io.Writer, limits *flex.JobLimits) (code int, err error) {
	const graceTime = 5 * time.Second

	timeLimit := limits.GetTime().AsDuration()
	ctx, cancel := context.WithTimeout(ctx, timeLimit+graceTime)
	defer cancel()

	args := cmd.GetArgs()
	if len(args) == 0 {
		return -1, errors.New("command is empty")
	}

	c := exec.CommandContext(ctx, args[0], args[1:]...)
	c.Dir = execDir
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = append(os.Environ(), "OUT_DIR="+outDir)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := c.Start(); err != nil {
		return -1, err
	}
	defer unix.Kill(-c.Process.Pid, unix.SIGKILL)

	timer := time.NewTimer(timeLimit)
	defer timer.Stop()
	go func() {
		select {
		case <-timer.C:
			c.Process.Signal(unix.SIGTERM)
		case <-ctx.Done():
		}
	}()

	if err := c.Wait(); err != nil {
		if errExit, ok := err.(*exec.ExitError); ok {
			if status, ok := errExit.Sys().(syscall.WaitStatus); ok {
				if status.Exited() {
					code := status.ExitStatus()
					return code, fmt.Errorf("exit status %d", code)
				}
				if status.Signaled() {
					sig := status.Signal()
					return 128 + int(sig), fmt.Errorf("signal: %s", unix.SignalName(sig))
				}
			}
		}
		return -1, err
	}
	return 0, nil
}
