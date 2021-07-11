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

package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nya3jp/flex/flexpb"
	"golang.org/x/sync/errgroup"
)

func runTask(ctx context.Context, task *flexpb.Task, rootDir string, stdout, stderr io.Writer) (code int, err error) {
	tasksDir := filepath.Join(rootDir, "tasks")
	cacheDir := filepath.Join(rootDir, "cache")
	for _, dir := range []string{tasksDir, cacheDir} {
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return -1, err
		}
	}

	taskDir, err := ioutil.TempDir(tasksDir, "")
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(taskDir)

	execDir := filepath.Join(taskDir, "exec")
	outDir := filepath.Join(taskDir, "out")
	for _, dir := range []string{execDir, outDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return -1, err
		}
	}

	if err := prepareTask(ctx, execDir, cacheDir, task.GetSpec().GetPackages()); err != nil {
		return -1, fmt.Errorf("failed to prepare task: %v", err)
	}

	code, err = execCmd(ctx, outDir, execDir, task.GetSpec().GetCommand(), stdout, stderr, task.GetSpec().GetLimits())
	if err != nil {
		return -1, fmt.Errorf("task execution failed: %v", err)
	}
	return code, nil
}

func prepareTask(ctx context.Context, execDir, cacheDir string, pkgs []*flexpb.TaskPackage) error {
	pkgCacheDir := filepath.Join(cacheDir, "pkgs")
	if err := os.MkdirAll(pkgCacheDir, 0700); err != nil {
		return err
	}

	pkgFiles := make([]*os.File, len(pkgs))
	defer func() {
		for _, f := range pkgFiles {
			if f != nil {
				f.Close()
			}
		}
	}()

	if err := func() error {
		eg, ctx := errgroup.WithContext(ctx)
		for i, pkg := range pkgs {
			i, pkg := i, pkg
			eg.Go(func() error {
				f, err := downloadPkg(ctx, pkg.GetUrl(), pkgCacheDir)
				if err != nil {
					return fmt.Errorf("failed to download %s: %v", pkg.GetUrl(), err)
				}
				pkgFiles[i] = f
				return nil
			})
		}
		return eg.Wait()
	}(); err != nil {
		return err
	}

	for i, pkg := range pkgs {
		if err := installPkg(ctx, filepath.Join(execDir, pkg.GetInstallPath()), pkgFiles[i]); err != nil {
			return fmt.Errorf("failed to stage %s: %v", pkg.GetUrl(), err)
		}
	}
	return nil
}

func downloadPkg(ctx context.Context, url, pkgCacheDir string) (*os.File, error) {
	cachePath := filepath.Join(pkgCacheDir, sha256sum(url))

	f, err := os.OpenFile(cachePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return f, nil
	}
	defer f.Close()

	dupFile := func() (*os.File, error) {
		nfd, err := syscall.Dup(int(f.Fd()))
		if err != nil {
			return nil, err
		}
		f := os.NewFile(uintptr(nfd), f.Name())
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	if pos, err := f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	} else if pos > 0 {
		// TODO: Deal with 0-byte case.
		return dupFile()
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}

	if pos, err := f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	} else if pos > 0 {
		// TODO: Deal with 0-byte case.
		return dupFile()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	w, err := dupFile()
	if err != nil {
		return nil, err
	}
	_, werr := io.Copy(w, res.Body)
	if err := w.Close(); err != nil && werr == nil {
		werr = err
	}

	if werr != nil {
		f.Truncate(0)
		return nil, werr
	}

	return dupFile()
}

func installPkg(ctx context.Context, destDir string, f *os.File) error {
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "tar", "xz", "-C", destDir)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar failed: %s: %v", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

func execCmd(ctx context.Context, outDir, execDir string, cmd *flexpb.TaskCommand, stdout, stderr io.Writer, limits *flexpb.TaskLimits) (code int, err error) {
	if tl := limits.GetTime().AsDuration(); tl > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, tl)
		defer cancel()
	}

	c := exec.CommandContext(ctx, "sh", "-c", cmd.GetShell())
	c.Dir = execDir
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = append(os.Environ(), "OUT_DIR="+outDir)
	if err := c.Run(); err != nil {
		if errExit, ok := err.(*exec.ExitError); ok {
			if status, ok := errExit.Sys().(syscall.WaitStatus); ok && status.Exited() {
				return status.ExitStatus(), nil
			}
		}
		return 0, err
	}
	return 0, nil
}

func sha256sum(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
