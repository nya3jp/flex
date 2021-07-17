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
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nya3jp/flex/cmd/flexlet/internal/flexlet"
	"github.com/nya3jp/flex/cmd/flexlet/internal/run"
	flexlet2 "github.com/nya3jp/flex/internal/flexlet"
	"google.golang.org/grpc"
)

type args struct {
	Name     string
	Workers  int
	HubAddr  string
	StoreDir string
}

func parseArgs() (*args, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	args := &args{}
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	fs.StringVar(&args.Name, "name", hostName, "Flexlet name")
	fs.IntVar(&args.Workers, "workers", runtime.NumCPU(), "Number of workers")
	fs.StringVar(&args.HubAddr, "hub", "", "Flexhub address")
	fs.StringVar(&args.StoreDir, "storedir", filepath.Join(homeDir, ".cache/flexlet"), "Storage directory path")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	if args.HubAddr == "" {
		return nil, errors.New("-hub is required")
	}
	return args, nil
}

func main() {
	if err := func() error {
		args, err := parseArgs()
		if err != nil {
			return err
		}

		ctx := context.Background()

		runner, err := run.New(args.StoreDir)
		if err != nil {
			return err
		}

		cc, err := grpc.DialContext(ctx, args.HubAddr, grpc.WithInsecure())
		if err != nil {
			return err
		}
		cl := flexlet2.NewFlexletServiceClient(cc)

		return flexlet.Run(ctx, cl, runner, args.Workers)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
