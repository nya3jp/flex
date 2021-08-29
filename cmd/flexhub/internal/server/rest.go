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
	"math"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/restfix"
	"github.com/nya3jp/flex/internal/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type restServer struct {
	cl     flex.FlexServiceClient
	engine *gin.Engine
}

func newRESTServer(cl flex.FlexServiceClient) *restServer {
	engine := gin.New()
	s := &restServer{cl: cl, engine: engine}
	engine.Use(cors.Default()) // allow all CORS requests
	engine.GET("/healthz", s.handleHealthz)
	engine.Use(static.Serve("/", static.LocalFile("./web", true)))
	api := engine.Group("/api")
	api.GET("/jobs", s.handleAPIJobs)
	api.GET("/jobs/:id", s.handleAPIJob)
	api.GET("/jobs/:id/stdout", s.handleAPIJobStdout)
	api.GET("/jobs/:id/stderr", s.handleAPIJobStderr)
	api.GET("/flexlets", s.handleAPIFlexlets)
	api.GET("/stats", s.handleAPIStats)
	return s
}

func (s *restServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

func (s *restServer) handleHealthz(ctx *gin.Context) {
	ctx.String(http.StatusOK, "ok")
}

type jobsRequest struct {
	Limit    int64  `form:"limit"`
	BeforeID int64  `form:"before"`
	Label    string `form:"label"`
}

func (s *restServer) handleAPIJobs(ctx *gin.Context) {
	respond(ctx, func() error {
		var req jobsRequest
		if err := ctx.ShouldBindQuery(&req); err != nil {
			return err
		}
		if req.Limit == 0 {
			req.Limit = math.MaxInt64
		}
		if req.BeforeID == 0 {
			req.BeforeID = math.MaxInt64
		}

		rpcReq := &flex.ListJobsRequest{
			Limit:    req.Limit,
			BeforeId: req.BeforeID,
			Label:    req.Label,
		}
		res, err := s.cl.ListJobs(ctx, rpcReq, withCreds(ctx))
		if err != nil {
			return err
		}

		for _, job := range res.GetJobs() {
			if err := restfix.JobStatus(job); err != nil {
				return err
			}
		}
		return writeProtoJSON(ctx, res)
	})
}

type jobRequest struct {
	ID int64 `uri:"id"`
}

func (s *restServer) handleAPIJob(ctx *gin.Context) {
	respond(ctx, func() error {
		var req jobRequest
		if err := ctx.ShouldBindUri(&req); err != nil {
			return err
		}

		rpcReq := &flex.GetJobRequest{
			Id: req.ID,
		}
		res, err := s.cl.GetJob(ctx, rpcReq, withCreds(ctx))
		if err != nil {
			return err
		}

		if err := restfix.JobStatus(res.GetJob()); err != nil {
			return err
		}
		return writeProtoJSON(ctx, res)
	})
}

func (s *restServer) handleAPIJobStdout(ctx *gin.Context) {
	s.handleAPIJobOutput(ctx, flex.GetJobOutputRequest_STDOUT)
}

func (s *restServer) handleAPIJobStderr(ctx *gin.Context) {
	s.handleAPIJobOutput(ctx, flex.GetJobOutputRequest_STDERR)
}

type jobOutputRequest struct {
	ID int64 `uri:"id"`
}

func (s *restServer) handleAPIJobOutput(ctx *gin.Context, outputType flex.GetJobOutputRequest_JobOutputType) {
	respond(ctx, func() error {
		var req jobOutputRequest
		if err := ctx.ShouldBindUri(&req); err != nil {
			return err
		}

		rpcReq := &flex.GetJobOutputRequest{
			Id:   req.ID,
			Type: outputType,
		}
		res, err := s.cl.GetJobOutput(ctx, rpcReq, withCreds(ctx))
		if err != nil {
			return err
		}
		ctx.Redirect(http.StatusFound, res.GetLocation().GetPresignedUrl())
		return nil
	})
}

func (s *restServer) handleAPIFlexlets(ctx *gin.Context) {
	respond(ctx, func() error {
		res, err := s.cl.ListFlexlets(ctx, &flex.ListFlexletsRequest{}, withCreds(ctx))
		if err != nil {
			return err
		}
		for _, flexlet := range res.GetFlexlets() {
			if err := restfix.FlexletStatus(flexlet); err != nil {
				return err
			}
		}
		return writeProtoJSON(ctx, res)
	})
}

func (s *restServer) handleAPIStats(ctx *gin.Context) {
	respond(ctx, func() error {
		res, err := s.cl.GetStats(ctx, &flex.GetStatsRequest{}, withCreds(ctx))
		if err != nil {
			return err
		}
		return writeProtoJSON(ctx, res)
	})
}

func respond(ctx *gin.Context, f func() error) {
	err := f()
	if err == nil {
		return
	}

	s, ok := status.FromError(err)
	if !ok {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	switch s.Code() {
	case codes.Unauthenticated:
		ctx.String(http.StatusUnauthorized, s.Message())
	case codes.PermissionDenied:
		ctx.String(http.StatusForbidden, s.Message())
	default:
		ctx.String(http.StatusInternalServerError, s.Message())
	}
}

func writeProtoJSON(ctx *gin.Context, msg proto.Message) error {
	ctx.Render(http.StatusOK, protoJSONRender{msg: msg})
	return nil
}

type protoJSONRender struct {
	msg proto.Message
}

var protoJSONOptions = protojson.MarshalOptions{
	EmitUnpopulated: true,
}

func (r protoJSONRender) Render(w http.ResponseWriter) error {
	b, err := protoJSONOptions.Marshal(r.msg)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (r protoJSONRender) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func withCreds(ctx *gin.Context) grpc.CallOption {
	return grpc.PerRPCCredentials(grpcutil.NewAuthCreds(ctx.GetHeader("authorization")))
}
