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
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/nya3jp/flex/internal/hashutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

type handler struct {
	tmpDir string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	hash := hex.EncodeToString(hashutil.NewStdHash().Sum([]byte(r.URL.Path)))
	path := filepath.Join(h.tmpDir, hash)
	if err := func() error {
		switch r.Method {
		case http.MethodGet:
			f, err := os.Open(path)
			if os.IsNotExist(err) {
				http.Error(w, "File not found", http.StatusNotFound)
				return nil
			}
			if err != nil {
				return err
			}
			defer f.Close()

			fi, err := f.Stat()
			if err != nil {
				return err
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
			_, err = io.CopyN(w, f, fi.Size())
			return err

		case http.MethodHead:
			fi, err := os.Stat(path)
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				return nil
			}
			if err != nil {
				return err
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
			w.WriteHeader(http.StatusOK)
			return nil

		case http.MethodPut:
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			_, firstErr := io.Copy(f, r.Body)
			if err := f.Close(); err != nil && firstErr == nil {
				firstErr = err
			}

			if firstErr != nil {
				return firstErr
			}
			w.WriteHeader(http.StatusNoContent)
			return nil

		default:
			http.Error(w, "method not supported", http.StatusBadRequest)
			return nil
		}
	}(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	app := &cli.App{
		Name:  "testfs",
		Usage: "Test File Storage Server",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Value: 8081, Usage: "TCP port to listen on"},
		},
		Action: func(c *cli.Context) error {
			ctx := c.Context
			port := c.Int("port")

			tmpDir, err := os.MkdirTemp("", "testfs.")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			server := http.Server{
				Addr:        fmt.Sprintf("0.0.0.0:%d", port),
				Handler:     &handler{tmpDir},
				BaseContext: func(net.Listener) context.Context { return ctx },
			}
			log.Printf("INFO: Listening at %s", server.Addr)
			go func() {
				<-ctx.Done()
				log.Print("INFO: Shutting down the server")
				server.Close()
			}()
			return server.ListenAndServe()
		},
	}
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
