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

package flex

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option"

	"github.com/nya3jp/flex/internal/unifs"
)

func TestRunTask(t *testing.T) {
	fs := unifs.New(context.Background(), option.WithoutAuthentication())

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
			td, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(td)

			task := &Task{
				Command: &TaskCommand{Shell: tc.cmd},
				Output:  &TaskOutput{Url: filepath.Join(td, "out")},
				Limits:  &TaskLimits{},
			}
			if tc.timeLimit {
				task.Limits.Time = ptypes.DurationProto(time.Nanosecond)
			}
			options := &RunTaskOptions{
				TaskDir:  filepath.Join(td, "task"),
				CacheDir: filepath.Join(td, "cache"),
				FS:       fs,
			}

			status, err := RunTask(context.Background(), task, options)
			if !tc.wantErr && err != nil {
				t.Fatalf("RunTask failed: %v", err)
			}
			if tc.wantErr && err == nil {
				t.Fatal("RunTask succeeded unexpectedly")
			}
			if err == nil && status != tc.wantStatus {
				t.Fatalf("RunTask returned %d, want %d", status, tc.wantStatus)
			}
		})
	}
}

func TestRunTaskEnv(t *testing.T) {
	fs := unifs.New(context.Background(), option.WithoutAuthentication())

	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	outDir := filepath.Join(td, "out")
	taskDir := filepath.Join(td, "task")
	task := &Task{
		Command: &TaskCommand{Shell: `echo "$OUT_DIR"`},
		Output:  &TaskOutput{Url: outDir},
	}
	options := &RunTaskOptions{
		TaskDir:  taskDir,
		CacheDir: filepath.Join(td, "cache"),
		FS:       fs,
	}

	status, err := RunTask(context.Background(), task, options)
	if err != nil {
		t.Fatalf("RunTask failed: %v", err)
	}
	if status != 0 {
		t.Fatalf("RunTask returned %d, want 0", status)
	}

	b, err := ioutil.ReadFile(filepath.Join(outDir, "stdout.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := strings.TrimSpace(string(b))
	// OUT_DIR should be somewhere under taskDir.
	if !strings.HasPrefix(got, taskDir) {
		t.Errorf("OUT_DIR=%q, want subdir of %q", got, taskDir)
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

func TestRunTaskPackaging(t *testing.T) {
	fs := unifs.New(context.Background(), option.WithoutAuthentication())

	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	pkg1 := filepath.Join(td, "pkg1.tar")
	pkg2 := filepath.Join(td, "pkg2.tar")
	writeTar(t, pkg1, []string{"file1", "dir2/file2"})
	writeTar(t, pkg2, []string{"file3", "dir1/file4"})

	outDir := filepath.Join(td, "out")
	task := &Task{
		Command: &TaskCommand{Shell: "find -s ."},
		Output:  &TaskOutput{Url: outDir},
		Packages: []*TaskPackage{
			{Url: pkg1, InstallPath: "dir1"},
			{Url: pkg2, InstallPath: ""},
		},
	}
	options := &RunTaskOptions{
		TaskDir:  filepath.Join(td, "task"),
		CacheDir: filepath.Join(td, "cache"),
		FS:       fs,
	}

	status, err := RunTask(context.Background(), task, options)
	if err != nil {
		t.Fatalf("RunTask failed: %v", err)
	}
	if status != 0 {
		t.Fatalf("RunTask returned %d, want 0", status)
	}

	b, err := ioutil.ReadFile(filepath.Join(outDir, "stdout.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(b)
	want := `.
./dir1
./dir1/dir2
./dir1/dir2/file2
./dir1/file1
./dir1/file4
./file3
`
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Files mismatch (-got +want):\n%s", diff)
	}
}
