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
	"fmt"
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
	"golang.org/x/sync/errgroup"
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
			Name:  "flexlet",
			Usage: "Flexlet",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "name", Value: hostName, Usage: "Flexlet name"},
				&cli.IntFlag{Name: "cores", Value: runtime.NumCPU(), Usage: "Number of available cores"},
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub address"},
				&cli.BoolFlag{Name: "insecure", Usage: "Use insecure connections to Flexhub servers"},
				&cli.StringFlag{Name: "storedir", Value: filepath.Join(homeDir, ".cache/flexlet"), Usage: "Storage directory path"},
				&cli.StringFlag{Name: "password", Usage: "Sets a Flexlet service password"},
				&cli.StringFlag{Name: "password-from-file", Usage: "Reads a Flexlet service password from a file"},
				&cli.IntFlag{Name: "replicas-for-load-testing", Value: 1, Hidden: true},
			},
			Action: func(c *cli.Context) error {
				name := c.String("name")
				cores := c.Int("cores")
				hubAddr := c.String("hub")
				insecure := c.Bool("insecure")
				storeDir := c.String("storedir")
				password := c.String("password")
				passwordFile := c.String("password-from-file")
				replicas := c.Int("replicas-for-load-testing")

				runner, err := run.New(storeDir)
				if err != nil {
					return err
				}

				cc, err := grpcutil.DialContext(ctx, hubAddr, insecure, password, passwordFile)
				if err != nil {
					return err
				}
				cl := flexletpb.NewFlexletServiceClient(cc)

				grp, ctx := errgroup.WithContext(ctx)
				for i := 0; i < replicas; i++ {
					flexletID := &flex.FlexletId{Name: name}
					if replicas > 1 {
						flexletID.Name += fmt.Sprintf(".%d", i)
					}
					grp.Go(func() error {
						return flexlet.Run(ctx, cl, runner, flexletID, cores)
					})
				}
				return grp.Wait()
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
