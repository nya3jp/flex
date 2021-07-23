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
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/internal/hashutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

type flexServer struct {
	flex.UnimplementedFlexServiceServer
	meta *database.MetaStore
	fs   FS
}

func newFlexServer(meta *database.MetaStore, fs FS) *flexServer {
	return &flexServer{
		meta: meta,
		fs:   fs,
	}
}

func (s *flexServer) SubmitJob(ctx context.Context, req *flex.SubmitJobRequest) (*flex.SubmitJobResponse, error) {
	if req == nil {
		req = &flex.SubmitJobRequest{}
	}
	if req.Spec == nil {
		req.Spec = &flex.JobSpec{}
	}
	if req.Spec.Limits == nil {
		req.Spec.Limits = &flex.JobLimits{}
	}
	if req.Spec.Limits.Time == nil {
		req.Spec.Limits.Time = durationpb.New(defaultTimeLimit)
	}

	for _, pkg := range req.GetSpec().GetInputs().GetPackages() {
		if err := resolvePackageId(ctx, s.meta, pkg.GetId()); err != nil {
			return nil, err
		}
	}

	id, err := s.meta.InsertJob(ctx, req.GetSpec())
	if err != nil {
		return nil, err
	}
	return &flex.SubmitJobResponse{Id: id}, nil
}

func (s *flexServer) CancelJob(ctx context.Context, req *flex.CancelJobRequest) (*flex.CancelJobResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *flexServer) GetJob(ctx context.Context, req *flex.GetJobRequest) (*flex.GetJobResponse, error) {
	task, err := s.meta.GetJob(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &flex.GetJobResponse{Job: task}, nil
}

func (s *flexServer) GetJobOutput(ctx context.Context, req *flex.GetJobOutputRequest) (*flex.GetJobOutputResponse, error) {
	status, err := s.meta.GetJob(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	var name string
	switch req.GetType() {
	case flex.GetJobOutputRequest_STDOUT:
		name = stdoutName
	case flex.GetJobOutputRequest_STDERR:
		name = stderrName
	default:
		return nil, fmt.Errorf("unknown output type: %d", req.GetType())
	}
	path := pathForTask(status.GetTaskId(), name)

	url, err := s.fs.PresignedURLForGet(ctx, path, time.Minute)
	if err != nil {
		return nil, err
	}

	loc := &flex.FileLocation{
		CanonicalUrl: s.fs.CanonicalURL(path),
		PresignedUrl: url,
	}
	return &flex.GetJobOutputResponse{Location: loc}, nil
}

func (s *flexServer) ListJobs(ctx context.Context, req *flex.ListJobsRequest) (*flex.ListJobsResponse, error) {
	tasks, err := s.meta.ListJobs(ctx, req.GetLimit(), req.GetBeforeId(), req.GetState())
	if err != nil {
		return nil, err
	}
	return &flex.ListJobsResponse{Jobs: tasks}, nil
}

func (s *flexServer) InsertPackage(stream flex.FlexService_InsertPackageServer) error {
	ctx := stream.Context()

	req, err := stream.Recv()
	if err != nil {
		return err
	}

	_, ok := req.GetType().(*flex.InsertPackageRequest_Spec)
	if !ok {
		return errors.New("protocol error: expected InsertPackageRequest_Spec")
	}

	f, err := os.CreateTemp("", "flexhub.package.")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	hasher := hashutil.NewTeeHasher(f, hashutil.NewStdHash())

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		hasher.Write(req.GetData())
	}

	hash := hasher.SumString()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if err := s.fs.Put(ctx, pathForPackage(hash), f); err != nil {
		return err
	}

	return stream.SendAndClose(&flex.InsertPackageResponse{Id: &flex.PackageId{Hash: hash}})
}

func (s *flexServer) GetPackage(ctx context.Context, req *flex.GetPackageRequest) (*flex.GetPackageResponse, error) {
	id := req.GetId()
	if err := resolvePackageId(ctx, s.meta, id); err != nil {
		return nil, err
	}
	err := s.fs.Exists(ctx, pathForPackage(id.GetHash()))
	if errors.Is(err, os.ErrNotExist) {
		return nil, status.Errorf(codes.NotFound, "package not found: %s: %v", id.GetHash(), err)
	}
	if err != nil {
		return nil, err
	}
	return &flex.GetPackageResponse{
		Package: &flex.Package{
			Id:   id,
			Spec: &flex.PackageSpec{},
		},
	}, nil
}

func (s *flexServer) UpdateTag(ctx context.Context, req *flex.UpdateTagRequest) (*flex.UpdateTagResponse, error) {
	tag := req.GetTag()
	if tag == "" {
		return nil, errors.New("tag is empty")
	}
	hash := req.GetHash()
	if !hashutil.IsStdHash(hash) {
		return nil, errors.New("invalid hash")
	}
	if err := s.meta.UpdateTag(ctx, tag, hash); err != nil {
		return nil, err
	}
	return &flex.UpdateTagResponse{}, nil
}

func (s *flexServer) ListTags(ctx context.Context, req *flex.ListTagsRequest) (*flex.ListTagsResponse, error) {
	ids, err := s.meta.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	return &flex.ListTagsResponse{Tags: ids}, nil
}

func (s *flexServer) ListFlexlets(ctx context.Context, req *flex.ListFlexletsRequest) (*flex.ListFlexletsResponse, error) {
	flexlets, err := s.meta.ListFlexlets(ctx)
	if err != nil {
		return nil, err
	}
	return &flex.ListFlexletsResponse{Flexlets: flexlets}, nil
}

func (s *flexServer) GetStats(ctx context.Context, req *flex.GetStatsRequest) (*flex.GetStatsResponse, error) {
	stats, err := s.meta.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	return &flex.GetStatsResponse{Stats: stats}, nil
}

func resolvePackageId(ctx context.Context, meta *database.MetaStore, id *flex.PackageId) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("resolving package ID: %w", err)
		}
	}()

	if tag := id.GetTag(); tag != "" {
		hash, err := meta.LookupTag(ctx, tag)
		if err != nil {
			return err
		}
		id.Hash = hash
	}
	if !hashutil.IsStdHash(id.GetHash()) {
		return errors.New("invalid package hash")
	}
	return nil
}
