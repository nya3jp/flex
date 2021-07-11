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
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexlet/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type args struct {
	Workers int
	HubAddr string
	Options worker.Options
}

func parseArgs() (*args, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	args := &args{}
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	fs.IntVar(&args.Workers, "workers", 0, "Number of workers")
	fs.StringVar(&args.HubAddr, "hub", "", "Flexhub address")
	fs.StringVar(&args.Options.RootDir, "storedir", filepath.Join(homeDir, ".cache/flexlet"), "Storage directory path")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	if args.HubAddr == "" {
		return nil, errors.New("-hub is required")
	}
	if args.Workers <= 0 {
		return nil, errors.New("-workers is required")
	}
	return args, nil
}

func runWorker(hubAddr string, options *worker.Options) error {
	conn, err := net.Dial("tcp", hubAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	srv := grpc.NewServer()
	flex.RegisterWorkerServer(srv, worker.New(options))
	reflection.Register(srv)

	return srv.Serve(newFixedListener(conn))
}

func main() {
	if err := func() error {
		args, err := parseArgs()
		if err != nil {
			return err
		}

		var wg sync.WaitGroup
		wg.Add(args.Workers)
		for i := 0; i < args.Workers; i++ {
			go func(i int) {
				defer wg.Done()
				if err := runWorker(args.HubAddr, &args.Options); err != nil {
					log.Printf("Worker %d failed: %v", i, err)
				}
			}(i)
		}
		wg.Wait()
		return nil
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
