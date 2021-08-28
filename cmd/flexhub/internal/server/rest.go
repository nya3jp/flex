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
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"google.golang.org/grpc"
)

type restServer struct {
	grpcServer *grpc.Server
	router     *httprouter.Router
}

func newRESTServer(grpcServer *grpc.Server) *restServer {
	s := &restServer{grpcServer: grpcServer, router: httprouter.New()}
	s.router.GET("/", s.handleOK)
	s.router.GET("/healthz", s.handleOK)
	return s
}

func (s *restServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *restServer) handleOK(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	io.WriteString(w, "ok")
}
