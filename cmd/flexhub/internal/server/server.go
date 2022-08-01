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
	"net/http"
	"strings"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/cmd/flexhub/internal/sentinel"
	"github.com/nya3jp/flex/internal/flexletpb"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func newDualHandler(grpcServer *grpc.Server, restServer http.Handler) http.Handler {
	splitHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		restServer.ServeHTTP(w, r)
	})
	h2cHandler := h2c.NewHandler(splitHandler, &http2.Server{})
	return h2cHandler
}

func Run(ctx context.Context, port int, meta *database.MetaStore, fs FS, password string, sentinel *sentinel.Sentinel) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}
	defer lis.Close()

	cc, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%d", port), grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer cc.Close()

	grpcServer := grpc.NewServer(makeAuthOptions(password)...)
	flex.RegisterFlexServiceServer(grpcServer, newFlexServer(meta, fs, sentinel))
	flexletpb.RegisterFlexletServiceServer(grpcServer, newFlexletServer(meta, fs))

	restServer := newRESTServer(flex.NewFlexServiceClient(cc))

	httpServer := &http.Server{
		Handler:     newDualHandler(grpcServer, restServer),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	log.Printf("INFO: Listening at %s", lis.Addr().String())

	go func() {
		<-ctx.Done()
		log.Print("INFO: Shutting down the server")
		httpServer.Close()
	}()
	return httpServer.Serve(lis)
}
