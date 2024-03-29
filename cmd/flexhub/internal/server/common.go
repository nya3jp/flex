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

package server

import (
	"context"
	"io"
	"path"
	"time"
)

const (
	defaultTimeLimit = time.Minute
	preTaskTime      = time.Minute
	postTaskTime     = time.Minute

	stdoutName = "stdout.txt"
	stderrName = "stderr.txt"
)

type FS interface {
	Exists(ctx context.Context, path string) error
	Put(ctx context.Context, path string, r io.ReadSeeker) error
	PresignedURLForGet(ctx context.Context, path string, dur time.Duration) (string, error)
	PresignedURLForPut(ctx context.Context, path string, dur time.Duration) (string, error)
	CanonicalURL(path string) string
}

func pathForPackage(hash string) string {
	return path.Join("packages", hash)
}

func pathForTask(id string, name string) string {
	return path.Join("tasks", id, name)
}
