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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/sync/errgroup"

	"github.com/nya3jp/flex"
)

type runTaskOptions struct {
	TaskDir  string
	CacheDir string
	FS       *uniFS
}

func runTask(ctx context.Context, task *flex.Task, opts *runTaskOptions) (code int, err error) {
	if err := os.MkdirAll(opts.TaskDir, 0700); err != nil {
		return -1, err
	}

	taskDir, err := ioutil.TempDir(opts.TaskDir, "")
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

	if err := prepareTask(ctx, opts.FS, execDir, opts.CacheDir, task.GetPackages()); err != nil {
		return -1, fmt.Errorf("failed to prepare task: %v", err)
	}

	code, err = execCmd(ctx, outDir, execDir, task.GetCommand(), task.GetLimits())
	if err != nil {
		return -1, fmt.Errorf("task execution failed: %v", err)
	}

	if err := uploadArtifacts(ctx, opts.FS, task.GetOutputs().GetBaseUrl(), outDir); err != nil {
		return -1, fmt.Errorf("failed to upload artifacts: %v", err)
	}
	return code, nil
}

func prepareTask(ctx context.Context, fs *uniFS, execDir, cacheDir string, pkgs []*flex.TaskPackage) error {
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
				f, err := downloadPkg(ctx, fs, pkg.GetUrl(), pkgCacheDir)
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

func downloadPkg(ctx context.Context, fs *uniFS, url, pkgCacheDir string) (*os.File, error) {
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

	r, err := fs.OpenForRead(ctx, url)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	w, err := dupFile()
	if err != nil {
		return nil, err
	}
	_, werr := io.Copy(w, r)
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

	cmd := exec.CommandContext(ctx, "tar", "x", "-z", "-C", destDir)
	cmd.Stdin = f
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar failed: %s: %v", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

func execCmd(ctx context.Context, outDir, execDir string, cmd *flex.TaskCommand, limits *flex.TaskLimits) (code int, err error) {
	stdout, err := os.Create(filepath.Join(outDir, "stdout.txt"))
	if err != nil {
		return 0, err
	}
	defer stdout.Close()

	stderr, err := os.Create(filepath.Join(outDir, "stderr.txt"))
	if err != nil {
		return 0, err
	}
	defer stderr.Close()

	if tlp := limits.GetTime(); tlp != nil {
		tl, err := ptypes.Duration(tlp)
		if err != nil {
			return 0, err
		}
		if tl > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, tl)
			defer cancel()
		}
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

func uploadArtifacts(ctx context.Context, fs *uniFS, outURL, outDir string) error {
	var paths []string
	if err := filepath.Walk(outDir, func(absPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		path := strings.TrimLeft(strings.TrimPrefix(absPath, outDir), "/")
		paths = append(paths, path)
		return nil
	}); err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, path := range paths {
		path := path
		eg.Go(func() error {
			src := filepath.Join(outDir, path)
			dst := outURL
			if !strings.HasSuffix(dst, "/") {
				dst += "/"
			}
			dst += path
			return uploadArtifact(ctx, fs, src, dst)
		})
	}
	return eg.Wait()
}

func uploadArtifact(ctx context.Context, fs *uniFS, src, dst string) (retErr error) {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	w, err := fs.OpenForWrite(ctx, dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := w.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if _, err := io.Copy(w, f); err != nil {
		cancel()
		return err
	}
	return nil
}

func sha256sum(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
