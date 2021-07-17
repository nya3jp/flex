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

package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/internal/flexlet"
	"google.golang.org/grpc"
)

func Run(ctx context.Context, port int, meta *database.MetaStore, fs FS) error {
	srv := grpc.NewServer()
	flex.RegisterFlexServiceServer(srv, newFlexServer(meta, fs))
	flexlet.RegisterFlexletServiceServer(srv, newFlexletServer(meta, fs))

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}

	log.Printf("INFO: Listening at 0.0.0.0:%d", port)

	go func() {
		<-ctx.Done()
		log.Print("INFO: Shutting down the server")
		srv.Stop()
	}()
	return srv.Serve(lis)
}
