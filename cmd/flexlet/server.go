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
	"net"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/nya3jp/flex"
)

type server struct {
	flex.UnimplementedFlexletServer
	opts *runTaskOptions
	lock chan struct{}
}

func newServer(opts *runTaskOptions) *server {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &server{
		opts: opts,
		lock: lock,
	}
}

func (s *server) RunTask(ctx context.Context, req *flex.RunTaskRequest) (*flex.RunTaskResponse, error) {
	select {
	case <-s.lock:
	default:
		return nil, status.Error(codes.ResourceExhausted, "another task is running")
	}
	defer func() { s.lock <- struct{}{} }()

	var result flex.TaskResult
	code, err := runTask(ctx, req.GetTask(), s.opts)
	if err != nil {
		result.Status = &flex.TaskResult_Error{Error: err.Error()}
	} else {
		result.Status = &flex.TaskResult_ExitCode{ExitCode: int32(code)}
	}
	return &flex.RunTaskResponse{Result: &result}, nil
}

func runServer(port int, options *runTaskOptions) error {
	srv := grpc.NewServer()

	flex.RegisterFlexletServer(srv, newServer(options))
	reflection.Register(srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)
	go func() {
		sig := <-ch
		log.Printf("Received %v, stopping now", sig)
		srv.GracefulStop()
	}()
	signal.Notify(ch, unix.SIGTERM, unix.SIGINT)

	log.Printf("Started listening at %v", lis.Addr())
	return srv.Serve(lis)
}
