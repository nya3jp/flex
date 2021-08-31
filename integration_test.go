// Copyright 2021 Google LLC
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

// +build integration

package flex_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func startCommand(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd, cmd.Start()
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

func waitHTTP(t *testing.T, port int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for ctx.Err() == nil {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://localhost:%d", port), nil)
		if err != nil {
			t.Fatal(err)
		}

		if res, err := http.DefaultClient.Do(req); err == nil {
			res.Body.Close()
			return
		}
	}
}

func setUp(t *testing.T) {
	homeDir := t.TempDir()
	binDir := filepath.Join(homeDir, "bin")
	if err := os.MkdirAll(binDir, 0777); err != nil {
		t.Fatal(err)
	}

	// Compile Go binaries.
	t.Log("Compiling Flex binaries...")
	goCmd := exec.Command(
		"go",
		"install",
		"./cmd/flex",
		"./cmd/flexhub",
		"./cmd/flexlet",
		"./cmd/testfs",
	)
	goCmd.Env = append(os.Environ(), "GOBIN=" + binDir)
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	if err := goCmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Set up environment variables.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir + ":" + oldPath)
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })

	// Set up MySQL test database.
	t.Log("Setting up MySQL test database...")
	dbURL := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true",
		os.Getenv("FLEX_TEST_DB_USER"),
		os.Getenv("FLEX_TEST_DB_PASS"),
		os.Getenv("FLEX_TEST_DB_HOST"),
		os.Getenv("FLEX_TEST_DB_NAME"),
	)
	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, table := range []string{"labels", "flexlets", "tags", "tasks", "jobs"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatalf("Failed to drop table %s: %v", table, err)
		}
	}

	// Configure Flex CLI.
	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "flex"), 0700); err != nil {
		t.Fatal(err)
	}
	const config = `[flex]
hub = http://localhost:57111/
password = foobar
`
	if err := os.WriteFile(filepath.Join(homeDir, ".config", "flex", "config.ini"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}

	// Start TestFS.
	t.Log("Starting TestFS...")
	fsCmd, err := startCommand("testfs", "--port=57180")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fsCmd.Process.Kill()
		fsCmd.Process.Wait()
	})
	waitHTTP(t, 57180)

	// Start Flexhub.
	t.Log("Starting Flexhub...")
	hubCmd, err := startCommand("flexhub", "--port=57111", "--db=" + dbURL, "--fs=http://localhost:57180/", "--password=foobar")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		hubCmd.Process.Kill()
		hubCmd.Process.Wait()
	})
	waitHTTP(t, 57111)

	t.Log("Finished setup")
}

type flexlet struct {
	t *testing.T
	cmd *exec.Cmd
}

func (f *flexlet) Stop() {
	f.t.Log("Stopping Flexlet...")
	_ = f.cmd.Process.Kill()
	_, _ = f.cmd.Process.Wait()
}

func startFlexlet(t *testing.T) *flexlet {
	t.Log("Starting Flexlet...")
	cmd, err := startCommand("flexlet", "--hub=http://localhost:57111/", "--password=foobar")
	if err != nil {
		t.Fatalf("Failed to start flexlet: %v", err)
	}
	return &flexlet{t: t, cmd: cmd}
}

func TestIntegration(t *testing.T) {
	setUp(t)

	func() {
		t.Log("******** Simple run test")

		f := startFlexlet(t)
		defer f.Stop()

		const msg = "Hello, world!"
		tempDir := t.TempDir()
		readmePath := filepath.Join(tempDir, "README.txt")
		if err := os.WriteFile(readmePath, []byte(msg), 0600); err != nil {
			t.Fatal(err)
		}

		out, err := runCommand("flex", "run", "-f", readmePath, "cat", "README.txt")
		if err != nil {
			t.Fatalf("flex job run: %v", err)
		}
		if !strings.Contains(out, msg) {
			t.Fatalf("flex job run: unexpected output: got %q, want %q as a substring", out, msg)
		}
	}()

	func() {
		t.Log("******** Stress run test")

		for i := 0; i < 3; i++ {
			f := startFlexlet(t)
			defer f.Stop()
		}

		const msg = "ok"
		const n = 30

		var ids []string
		for i := 0; i < n; i++ {
			out, err := runCommand("flex", "job", "create", "echo", msg)
			if err != nil {
				t.Fatalf("flex job create: %v", err)
			}
			ids = append(ids, strings.TrimSpace(out))
		}

		for _, id := range ids {
			if _, err := runCommand("flex", "job", "wait", id); err != nil {
				t.Fatalf("flex job wait %s: %v", id, err)
			}
			out, err := runCommand("flex", "job", "outputs", id)
			if err != nil {
				t.Fatalf("flex job outputs %s: %v", id, err)
			}
			if got := strings.TrimSpace(out); got != msg {
				t.Fatalf("flex job outputs %s: got %q, want %q", id, got, msg)
			}
		}
	}()
}
