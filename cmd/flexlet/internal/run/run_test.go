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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	"github.com/nya3jp/flex/internal/flexlet"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
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
		spec *flexlet.TaskSpec
		want *flex.JobResult
	}{
		{
			spec: &flexlet.TaskSpec{
				Command: &flex.JobCommand{Shell: "true"},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.JobResult{
				Status: &flex.JobResult_ExitCode{ExitCode: 0},
			},
		},
		{
			spec: &flexlet.TaskSpec{
				Command: &flex.JobCommand{Shell: "exit 28"},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.JobResult{
				Status: &flex.JobResult_ExitCode{ExitCode: 28},
			},
		},
		{
			spec: &flexlet.TaskSpec{
				Command: &flex.JobCommand{Shell: "kill -INT $$"},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Minute)},
			},
			want: &flex.JobResult{
				Status: &flex.JobResult_Error{Error: "task execution failed: signal: SIGINT"},
			},
		},
		{
			spec: &flexlet.TaskSpec{
				Command: &flex.JobCommand{Shell: "sleep 60"},
				Limits:  &flex.JobLimits{Time: durationpb.New(time.Nanosecond)},
			},
			want: &flex.JobResult{
				Status: &flex.JobResult_Error{Error: "task execution failed: timeout reached (1ns)"},
			},
		},
	} {
		t.Run(tc.spec.GetCommand().GetShell(), func(t *testing.T) {
			got := runner.RunTask(context.Background(), tc.spec)
			if diff := cmp.Diff(got, tc.want, protocmp.Transform(), protocmp.IgnoreFields(&flex.JobResult{}, "time")); diff != "" {
				t.Fatalf("JobResult mismatch (-got +want):\n%s", diff)
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

	spec := &flexlet.TaskSpec{
		Command: &flex.JobCommand{Shell: "find -s ."},
		Inputs: &flexlet.TaskInputs{
			Packages: []*flexlet.TaskPackage{
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
		Outputs: &flexlet.TaskOutputs{
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

	spec := &flexlet.TaskSpec{
		Command: &flex.JobCommand{Shell: `echo foo; echo bar >&2`},
		Outputs: &flexlet.TaskOutputs{
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
