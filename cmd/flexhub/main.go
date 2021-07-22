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
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/cmd/flexhub/internal/filestorage"
	"github.com/nya3jp/flex/cmd/flexhub/internal/server"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

func newFileSystem(ctx context.Context, fsURL string) (server.FS, error) {
	parsed, err := url.Parse(fsURL)
	if err != nil {
		return nil, err
	}
	switch parsed.Scheme {
	case "gs":
		return filestorage.NewGS(ctx, fsURL)
	case "s3":
		return filestorage.NewS3(ctx, fsURL)
	case "http":
		return filestorage.NewAnonymous(fsURL)
	default:
		return nil, fmt.Errorf("unknown filesystem scheme: %s", parsed.Scheme)
	}
}

func run(ctx context.Context, port int, dbURL, fsURL string) error {
	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	meta := database.NewMetaStore(db)
	if err := meta.InitTables(ctx); err != nil {
		return err
	}
	if err := meta.Maintain(ctx); err != nil {
		return err
	}

	go func() {
		for {
			if err := ctxutil.Sleep(ctx, 10*time.Second); err != nil {
				return
			}
			if err := meta.Maintain(ctx); err != nil {
				log.Printf("WARNING: Table maintainance failed: %v", err)
			}
		}
	}()

	fs, err := newFileSystem(ctx, fsURL)
	if err != nil {
		return err
	}

	return server.Run(ctx, port, meta, fs)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	defaultPort := 7111
	if port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		defaultPort = port
	}

	app := &cli.App{
		Name:  "flexhub",
		Usage: "Flexhub",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Value: defaultPort, Usage: "TCP port to listen on"},
			&cli.StringFlag{Name: "db", Required: true, Usage: `DB URL (ex. "username:password@tcp(hostname:port)/database?parseTime=true")`},
			&cli.StringFlag{Name: "fs", Required: true, Usage: "File storage URL"},
		},
		Action: func(c *cli.Context) error {
			return run(c.Context, c.Int("port"), c.String("db"), c.String("fs"))
		},
	}
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
