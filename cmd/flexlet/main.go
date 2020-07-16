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

	"google.golang.org/api/option"
)

type args struct {
	port    int
	options *runTaskOptions
}

func parseArgs() (*args, error) {
	args := args{options: &runTaskOptions{}}
	var storeDir string
	var credsPath string
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	fs.IntVar(&args.port, "port", 2800, "Port to listen")
	fs.StringVar(&storeDir, "storedir", "", "Storage directory path")
	fs.StringVar(&credsPath, "creds", "", "Credentials JSON path")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	if storeDir == "" {
		return nil, errors.New("-storedir is required")
	}
	args.options.TaskDir = filepath.Join(storeDir, "task")
	args.options.CacheDir = filepath.Join(storeDir, "cache")

	var opts []option.ClientOption
	if credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credsPath))
	}
	args.options.FS = newUniFS(context.Background(), opts...)
	return &args, nil
}

func main() {
	if err := func() error {
		args, err := parseArgs()
		if err != nil {
			return err
		}
		return runServer(args.port, args.options)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
