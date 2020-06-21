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
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/storage"

	"github.com/nya3jp/flex/internal/concurrency"
	"github.com/nya3jp/flex/internal/flexlet"
)

const (
	workRoot = "/work"
)

var (
	apiKey string
)

func main() {
	if err := func() error {
		apiKey = os.Getenv("FLEX_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("$FLEX_API_KEY is not set")
		}

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			return fmt.Errorf("$PORT must be set correctly: %v", err)
		}
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			return fmt.Errorf("failed to listen: %v", err)
		}

		http.HandleFunc("/exec", execHandler)

		fmt.Fprintln(os.Stderr, "Ready")
		return http.Serve(l, nil)
	}(); err != nil {
		panic(fmt.Sprint("ERROR: ", err))
	}
}

func execHandler(w http.ResponseWriter, r *http.Request) {
	handle(w, r, func() error {
		if r.Method != http.MethodPost {
			return errors.New("unsupported method")
		}
		if k := r.Header.Get("X-API-Key"); k != apiKey {
			return errors.New("unauthorized")
		}

		var task flexlet.Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			return err
		}

		res := execTask(r.Context(), &task)

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

func execTask(ctx context.Context, task *flexlet.Task) *flexlet.TaskResult {
	var res *flexlet.TaskResult

	if err := func() error {
		if task.Timeout <= 0 || task.Timeout > 10*time.Minute {
			return errors.New("invalid timeout")
		}

		execDir, err := cleanDir("exec")
		if err != nil {
			return err
		}

		outDir, err := cleanDir("out")
		if err != nil {
			return err
		}

		cl, err := storage.NewClient(ctx)
		if err != nil {
			return err
		}

		if err := prepareTask(ctx, cl, execDir, task.Pkgs); err != nil {
			return fmt.Errorf("failed to prepare task: %v", err)
		}

		execCtx, cancel := context.WithTimeout(ctx, task.Timeout)
		defer cancel()
		started := time.Now()
		code, err := execCmd(execCtx, outDir, execDir, task.Cmd)
		if err != nil {
			return fmt.Errorf("task execution failed: %v", err)
		}
		finished := time.Now()

		res = &flexlet.TaskResult{
			Task:     *task,
			Started:  started,
			Finished: finished,
			Code:     code,
		}

		if err := saveResult(outDir, res); err != nil {
			return fmt.Errorf("failed to save result.json: %v", err)
		}
		if err := uploadArtifacts(ctx, cl, task.ArtifactsURL, outDir); err != nil {
			return fmt.Errorf("failed to upload artifacts: %v", err)
		}
		return nil
	}(); err != nil {
		return newErrorTaskResult(task, err)
	}
	return res
}

func prepareTask(ctx context.Context, cl *storage.Client, execDir string, pkgs []*flexlet.Pkg) error {
	cacheDir := filepath.Join(workRoot, "cache")
	evictDir := filepath.Join(workRoot, "cache.evict")
	if err := os.RemoveAll(evictDir); err != nil {
		return err
	}
	if err := os.Rename(cacheDir, evictDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(evictDir, 0700); err != nil {
			return err
		}
	}
	defer os.RemoveAll(evictDir)

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return err
	}

	pkgMap := make(map[string]string)
	for _, pkg := range pkgs {
		hash := sha256sum(pkg.URL)
		cachePath := filepath.Join(cacheDir, hash)
		pkgMap[pkg.URL] = cachePath
		if err := os.Rename(filepath.Join(evictDir, hash), cachePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := os.RemoveAll(evictDir); err != nil {
		return err
	}

	r := concurrency.NewRunner()
	for url, path := range pkgMap {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		url, path := url, path
		r.Add(func(ctx context.Context) error {
			if err := downloadPkg(ctx, cl, url, path); err != nil {
				return fmt.Errorf("failed to download %s: %v", url, err)
			}
			return nil
		})
	}
	if err := r.Run(ctx); err != nil {
		return err
	}

	for _, pkg := range pkgs {
		if err := installPkg(ctx, filepath.Join(execDir, pkg.Dest), pkgMap[pkg.URL]); err != nil {
			return fmt.Errorf("failed to stage %s: %v", pkg.URL, err)
		}
	}
	return nil
}

func downloadPkg(ctx context.Context, cl *storage.Client, url, destPath string) error {
	bucket, path, err := parseGSURL(url)
	if err != nil {
		return err
	}

	tmpPath := destPath + ".download"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	defer os.Remove(tmpPath)

	if err := func() (retErr error) {
		defer func() {
			if err := f.Close(); err != nil && retErr == nil {
				retErr = err
			}
		}()

		r, err := cl.Bucket(bucket).Object(path).NewReader(ctx)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(f, r)
		return err
	}(); err != nil {
		return err
	}

	return os.Rename(tmpPath, destPath)
}

func installPkg(ctx context.Context, destDir, tarPath string) error {
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "tar", "x", "-z", "-f", tarPath, "-C", destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar failed: %s: %v", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

func execCmd(ctx context.Context, outDir, execDir, cmd string) (int, error) {
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

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Dir = execDir
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = append(os.Environ(), "OUT_DIR="+outDir)
	if err := c.Run(); err != nil {
		if errExit, ok := err.(*exec.ExitError); ok {
			if status, ok := errExit.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
		}
		return 0, err
	}
	return 0, nil
}

func saveResult(outDir string, res *flexlet.TaskResult) error {
	f, err := os.Create(filepath.Join(outDir, "result.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(res)
}

func uploadArtifacts(ctx context.Context, cl *storage.Client, artifactsURL, outDir string) error {
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

	r := concurrency.NewRunner()
	for _, path := range paths {
		path := path
		r.Add(func(ctx context.Context) error {
			return uploadArtifact(ctx, cl, artifactsURL, outDir, path)
		})
	}
	return r.Run(ctx)
}

func uploadArtifact(ctx context.Context, cl *storage.Client, artifactsURL, outDir, path string) (retErr error) {
	f, err := os.Open(filepath.Join(outDir, path))
	if err != nil {
		return err
	}
	defer f.Close()

	bucket, prefix, err := parseGSURL(artifactsURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rw := cl.Bucket(bucket).Object(prefix + path).NewWriter(ctx)
	defer func() {
		if err := rw.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	// TODO: Auto-detect MIME type.
	rw.ContentType = "application/octet-stream"
	rw.ContentEncoding = "gzip"

	gw := gzip.NewWriter(rw)

	if _, err := io.Copy(gw, f); err != nil {
		cancel()
		return err
	}
	if err := gw.Close(); err != nil {
		cancel()
		return err
	}
	return nil
}

func newErrorTaskResult(task *flexlet.Task, err error) *flexlet.TaskResult {
	return &flexlet.TaskResult{
		Task:  *task,
		Code:  -1,
		Error: err.Error(),
	}
}

func cleanDir(name string) (string, error) {
	dir := filepath.Join(workRoot, name)
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func parseGSURL(rawURL string) (bucket, path string, err error) {
	p, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	if p.Scheme != "gs" {
		return "", "", errors.New("not a GS URL")
	}
	return p.Host, strings.TrimPrefix(p.Path, "/"), nil
}

func sha256sum(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
