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

package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nya3jp/flex/cmd/flexhub/internal/server"
	"github.com/nya3jp/flex/cmd/flexhub/internal/taskqueue"
)

type args struct {
	Port         int
	DB           string
	ArtifactsURL *url.URL
}

func parseArgs() (*args, error) {
	args := &args{}
	var artifactsURL string
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	fs.IntVar(&args.Port, "port", 0, "TCP port for worker connections")
	fs.StringVar(&args.DB, "db", "", "DB URL")
	fs.StringVar(&artifactsURL, "artifacts", "", "GCS URL to upload artifacts")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if args.Port <= 0 || args.DB == "" || artifactsURL == "" {
		return nil, errors.New("-db and -port and -artifacts are required")
	}
	parsed, err := url.Parse(artifactsURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "gs" {
		return nil, errors.New("-artifacts should start with gs://")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		return nil, errors.New("-artifacts should end with /")
	}
	args.ArtifactsURL = parsed
	return args, nil
}

func main() {
	if err := func() error {
		ctx := context.Background()

		args, err := parseArgs()
		if err != nil {
			return err
		}

		gcs, err := storage.NewClient(ctx)
		if err != nil {
			return err
		}
		defer gcs.Close()

		bucket := gcs.Bucket(args.ArtifactsURL.Host)
		object := bucket.Object(strings.TrimPrefix(args.ArtifactsURL.Path, "/") + "write_test")
		if err := object.NewWriter(ctx).Close(); err != nil {
			return fmt.Errorf("failed verifying GCS write access: %w", err)
		}

		db, err := sql.Open("mysql", args.DB)
		if err != nil {
			return err
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			return err
		}

		tq := taskqueue.New(db)
		return server.Run(ctx, args.Port, tq, gcs, args.ArtifactsURL)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
