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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/nya3jp/flex"
)

func TestRunTask(t *testing.T) {
	for _, tc := range []struct {
		cmd        string
		timeLimit  bool
		wantStatus int
		wantErr    bool
	}{
		{cmd: "true"},
		{cmd: "exit 28", wantStatus: 28},
		{cmd: "kill -INT $$", wantErr: true},
		{cmd: "sleep 60", timeLimit: true, wantErr: true},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			rootDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(rootDir)

			task := &flex.Task{
				Command: &flex.TaskCommand{Shell: tc.cmd},
				Limits:  &flex.TaskLimits{},
			}
			if tc.timeLimit {
				task.Limits.Time = ptypes.DurationProto(time.Nanosecond)
			}

			status, err := runTask(context.Background(), task, rootDir, io.Discard, io.Discard)
			if !tc.wantErr && err != nil {
				t.Fatalf("runTask failed: %v", err)
			}
			if tc.wantErr && err == nil {
				t.Fatal("runTask succeeded unexpectedly")
			}
			if err == nil && status != tc.wantStatus {
				t.Fatalf("runTask returned %d, want %d", status, tc.wantStatus)
			}
		})
	}
}

func TestRunTask_Stdio(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	task := &flex.Task{
		Command: &flex.TaskCommand{Shell: `echo foo; echo bar >&2`},
	}

	var stdout, stderr bytes.Buffer
	status, err := runTask(context.Background(), task, rootDir, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}
	if status != 0 {
		t.Fatalf("runTask returned %d, want 0", status)
	}

	if out := stdout.String(); out != "foo\n" {
		t.Errorf("Unexpected stdout: got %q, want %q", out, "foo\n")
	}
	if out := stderr.String(); out != "bar\n" {
		t.Errorf("Unexpected stderr: got %q, want %q", out, "bar\n")
	}
}

func writeTar(t *testing.T, name string, files []string) {
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	gw := gzip.NewWriter(f)
	defer func() {
		if err := gw.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	for _, file := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: file,
			Size: 0,
			Mode: 0666,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunTask_Packaging(t *testing.T) {
	rootDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootDir)

	webDir := filepath.Join(rootDir, "web")
	if err := os.Mkdir(webDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeTar(t, filepath.Join(webDir, "pkg1.tar"), []string{"file1", "dir2/file2"})
	writeTar(t, filepath.Join(webDir, "pkg2.tar"), []string{"file3", "dir1/file4"})

	server := httptest.NewServer(http.FileServer(http.Dir(webDir)))
	defer server.Close()

	task := &flex.Task{
		Command: &flex.TaskCommand{Shell: "find -s ."},
		Packages: []*flex.TaskPackage{
			{Url: server.URL + "/pkg1.tar", InstallPath: "dir1"},
			{Url: server.URL + "/pkg2.tar", InstallPath: ""},
		},
	}

	var stdout bytes.Buffer
	status, err := runTask(context.Background(), task, rootDir, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}
	if status != 0 {
		t.Fatalf("runTask returned %d, want 0", status)
	}

	const want = `.
./dir1
./dir1/dir2
./dir1/dir2/file2
./dir1/file1
./dir1/file4
./file3
`
	if diff := cmp.Diff(stdout.String(), want); diff != "" {
		t.Errorf("Files mismatch (-got +want):\n%s", diff)
	}
}
