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
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/flexlet"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	"github.com/nya3jp/flex/internal/flexletpb"
	"github.com/nya3jp/flex/internal/grpcutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

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
			Name:  "flexhub",
			Usage: "Flexhub",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Value: hostName, Usage: "Flexlet name"},
				&cli.IntFlag{Name: "workers", Value: runtime.NumCPU(), Usage: "Number of workers"},
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub address"},
				&cli.BoolFlag{Name: "insecure", Usage: "Use insecure connections to Flexhub servers"},
				&cli.StringFlag{Name: "storedir", Value: filepath.Join(homeDir, ".cache/flexlet"), Usage: "Storage directory path"},
			},
			Action: func(c *cli.Context) error {
				name := c.String("name")
				workers := c.Int("workers")
				hubAddr := c.String("hub")
				insecure := c.Bool("insecure")
				storeDir := c.String("storedir")

				runner, err := run.New(storeDir)
				if err != nil {
					return err
				}

				cc, err := grpcutil.DialContext(ctx, hubAddr, insecure)
				if err != nil {
					return err
				}
				cl := flexletpb.NewFlexletServiceClient(cc)

				flexletID := &flex.FlexletId{Name: name}

				return flexlet.Run(ctx, cl, runner, flexletID, workers)
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
