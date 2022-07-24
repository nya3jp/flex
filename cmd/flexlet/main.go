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
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/nya3jp/flex/cmd/flexlet/internal/flexlet"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/grpcutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

func runInPullMode(ctx context.Context, name string, replicas, cores int, cl flexletpb.FlexletServiceClient, runner *run.Runner) error {
	grp, ctx := errgroup.WithContext(ctx)
	for i := 0; i < replicas; i++ {
		replicaName := name
		if replicas > 1 {
			replicaName += fmt.Sprintf(".%d", i)
		}
		grp.Go(func() error {
			return flexlet.RunInPullMode(ctx, cl, runner, replicaName, cores)
		})
	}
	return grp.Wait()
}

func runInPushMode(ctx context.Context, name string, cores int, cl flexletpb.FlexletServiceClient, runner *run.Runner) error {
	tokens := make(chan struct{}, cores)
	for i := 0; i < cores; i++ {
		tokens <- struct{}{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-tokens:
		default:
			http.Error(w, "ERROR: max parallelism reached", http.StatusInternalServerError)
			return
		}
		defer func() {
			tokens <- struct{}{}
		}()

		if err := flexlet.RunInPushMode(ctx, cl, runner, name); err != nil {
			http.Error(w, fmt.Sprintf("ERROR: %v", err), http.StatusInternalServerError)
			return
		}
		io.WriteString(w, "OK")
	})
	return http.ListenAndServe(":"+os.Getenv("PORT"), mux)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	if err := func() error {
		hostName, err := os.Hostname()
		if err != nil {
			return err
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		app := &cli.App{
			Name:  "flexlet",
			Usage: "Flexlet",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Value: hostName, Usage: "Flexlet name"},
				&cli.IntFlag{Name: "cores", Value: runtime.NumCPU(), Usage: "Number of available cores"},
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub URL"},
				&cli.StringFlag{Name: "storedir", Value: filepath.Join(homeDir, ".cache/flexlet"), Usage: "Storage directory path"},
				&cli.StringFlag{Name: "password", Usage: "Sets a Flexlet service password"},
				&cli.BoolFlag{Name: "push", Usage: "Run in push mode"},
				&cli.IntFlag{Name: "replicas-for-load-testing", Value: 1, Hidden: true},
			},
			Action: func(c *cli.Context) error {
				name := c.String("name")
				cores := c.Int("cores")
				hubURL := c.String("hub")
				storeDir := c.String("storedir")
				password := c.String("password")
				push := c.Bool("push")
				replicas := c.Int("replicas-for-load-testing")

				runner, err := run.New(storeDir)
				if err != nil {
					return err
				}

				cc, err := grpcutil.DialContext(ctx, hubURL, password)
				if err != nil {
					return err
				}
				cl := flexletpb.NewFlexletServiceClient(cc)

				if push {
					if replicas != 1 {
						return errors.New("replicas not supported in push mode")
					}
					return runInPushMode(ctx, name, cores, cl, runner)
				}
				return runInPullMode(ctx, name, replicas, cores, cl, runner)
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
