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

package run_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	"github.com/nya3jp/flex/internal/flexletpb"
)

func TestRunner_RunTask(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runner, err := run.New(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		spec *flexletpb.TaskSpec
		want *flex.TaskResult
	}{
		{
			spec: &flexletpb.TaskSpec{
				Command: &flex.JobCommand{Args: []string{"true"}},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.TaskResult{
				ExitCode: 0,
				Message:  "success",
			},
		},
		{
			spec: &flexletpb.TaskSpec{
				Command: &flex.JobCommand{Args: []string{"sh", "-e", "-c", "exit 28"}},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.TaskResult{
				ExitCode: 28,
				Message:  "exit status 28",
			},
		},
		{
			spec: &flexletpb.TaskSpec{
				Command: &flex.JobCommand{Args: []string{"sh", "-e", "-c", "kill -INT $$"}},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.TaskResult{
				ExitCode: int32(128 + unix.SIGINT),
				Message:  "signal: SIGINT",
			},
		},
		{
			spec: &flexletpb.TaskSpec{
				Command: &flex.JobCommand{Args: []string{"sleep", "60"}},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Nanosecond)},
			},
			want: &flex.TaskResult{
				ExitCode: int32(128 + unix.SIGTERM),
				Message:  "signal: SIGTERM",
			},
		},
	} {
		t.Run(strings.Join(tc.spec.GetCommand().GetArgs(), " "), func(t *testing.T) {
			got := runner.RunTask(context.Background(), tc.spec)
			if diff := cmp.Diff(got, tc.want, protocmp.Transform(), protocmp.IgnoreFields(&flex.TaskResult{}, "time")); diff != "" {
				t.Fatalf("TaskResult mismatch (-got +want):\n%s", diff)
			}
		})
	}
}

func TestRunner_RunTask_Inputs(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runner, err := run.New(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	webDir := filepath.Join(tempDir, "web")
	if err := os.Mkdir(webDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeTarGz(t, filepath.Join(webDir, "pkg1.tar.gz"), []string{"file1", "dir2/file2"})
	writeTarGz(t, filepath.Join(webDir, "pkg2.tar.gz"), []string{"file3", "dir1/file4"})

	server := httptest.NewServer(http.FileServer(http.Dir(webDir)))
	defer server.Close()

	stdout, err := os.CreateTemp(tempDir, "stdout.")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()

	spec := &flexletpb.TaskSpec{
		Command: &flex.JobCommand{Args: []string{"sh", "-e", "-c", "find . | sort"}},
		Inputs: &flexletpb.TaskInputs{
			Packages: []*flexletpb.TaskPackage{
				{
					Location: &flex.FileLocation{
						CanonicalUrl: server.URL + "/pkg1.tar.gz",
						PresignedUrl: server.URL + "/pkg1.tar.gz",
					},
					InstallDir: "dir1",
				},
				{
					Location: &flex.FileLocation{
						CanonicalUrl: server.URL + "/pkg2.tar.gz",
						PresignedUrl: server.URL + "/pkg2.tar.gz",
					},
				},
			},
		},
		Outputs: &flexletpb.TaskOutputs{
			Stdout: &flex.FileLocation{
				CanonicalUrl: "file://" + stdout.Name(),
				PresignedUrl: "file://" + stdout.Name(),
			},
		},
		Limits: &flex.JobLimits{Time: durationpb.New(time.Minute)},
	}

	_ = runner.RunTask(context.Background(), spec)

	out, _ := io.ReadAll(stdout)
	const want = `.
./dir1
./dir1/dir2
./dir1/dir2/file2
./dir1/file1
./dir1/file4
./file3
`
	if diff := cmp.Diff(string(out), want); diff != "" {
		t.Errorf("Files mismatch (-got +want):\n%s", diff)
	}
}

func TestRunner_RunTask_Outputs(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runner, err := run.New(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	stdout, err := os.CreateTemp(tempDir, "stdout.")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()

	stderr, err := os.CreateTemp(tempDir, "stderr.")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	spec := &flexletpb.TaskSpec{
		Command: &flex.JobCommand{Args: []string{"sh", "-e", "-c", "echo foo; echo bar >&2"}},
		Outputs: &flexletpb.TaskOutputs{
			Stdout: &flex.FileLocation{
				CanonicalUrl: "file://" + stdout.Name(),
				PresignedUrl: "file://" + stdout.Name(),
			},
			Stderr: &flex.FileLocation{
				CanonicalUrl: "file://" + stderr.Name(),
				PresignedUrl: "file://" + stderr.Name(),
			},
		},
		Limits: &flex.JobLimits{Time: durationpb.New(time.Minute)},
	}

	_ = runner.RunTask(context.Background(), spec)

	if b, _ := io.ReadAll(stdout); string(b) != "foo\n" {
		t.Errorf("Unexpected stdout: got %q, want %q", string(b), "foo\n")
	}
	if b, _ := io.ReadAll(stderr); string(b) != "bar\n" {
		t.Errorf("Unexpected stderr: got %q, want %q", string(b), "bar\n")
	}
}

func writeTarGz(t *testing.T, name string, files []string) {
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
