// Copyright 2022 Google LLC
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
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/concurrent"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/flexletutil"
	"github.com/nya3jp/flex/internal/grpcutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

func run(ctx context.Context, name string, cl flexletpb.FlexletServiceClient, flexletURL string, parallelism int, interval time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go flexletutil.RunFletletUpdater(ctx, cl, &flex.Flexlet{Name: name, Spec: &flex.FlexletSpec{Cores: int32(parallelism)}})

	limiter := concurrent.NewLimiter(parallelism)
	retry := concurrent.NewRetry(time.Second, time.Minute)

	for {
		if err := ctxutil.Sleep(ctx, interval); err != nil {
			return err
		}

		res, err := cl.CountPendingTasks(ctx, &flexletpb.CountPendingTasksRequest{})
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}

		for remaining := res.GetCount(); remaining > 0; remaining-- {
			if !limiter.TryTake() {
				break
			}

			go func() {
				defer limiter.Done()

				if err := func() error {
					res, err := http.Post(flexletURL, "application/octet-stream", &bytes.Buffer{})
					if err != nil {
						return err
					}
					res.Body.Close()
					if res.StatusCode/100 != 2 {
						return fmt.Errorf("http status %d", res.StatusCode)
					}
					return nil
				}(); err != nil {
					log.Printf("ERROR: %v", err)
					retry.Wait(ctx)
					return
				}
				retry.Clear()
			}()
		}
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	if err := func() error {
		app := &cli.App{
			Name:  "flexbot",
			Usage: "Flexbot",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Required: true, Usage: "Flexbot name"},
				&cli.StringFlag{Name: "flexlet", Required: true, Usage: "Flexlet URL"},
				&cli.IntFlag{Name: "parallelism", Required: true, Usage: "Number of flexlets run in parallel"},
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub URL"},
				&cli.StringFlag{Name: "password", Required: true, Usage: "Sets a Flexlet service password"},
				&cli.DurationFlag{Name: "interval", Value: time.Second, Usage: "Polling interval"},
			},
			Action: func(c *cli.Context) error {
				name := c.String("name")
				parallelism := c.Int("parallelism")
				flexletURL := c.String("flexlet")
				hubURL := c.String("hub")
				password := c.String("password")
				interval := c.Duration("interval")

				cc, err := grpcutil.DialContext(ctx, hubURL, password)
				if err != nil {
					return err
				}
				cl := flexletpb.NewFlexletServiceClient(cc)

				return run(ctx, name, cl, flexletURL, parallelism, interval)
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
