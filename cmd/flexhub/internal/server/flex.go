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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flexhub/internal/database"
	"github.com/nya3jp/flex/internal/hashutil"
	"github.com/nya3jp/flex/internal/pubsub"
)

type flexServer struct {
	flex.UnimplementedFlexServiceServer
	meta      *database.MetaStore
	fs        FS
	publisher *pubsub.Publisher
}

func newFlexServer(meta *database.MetaStore, fs FS, publisher *pubsub.Publisher) *flexServer {
	return &flexServer{
		meta:      meta,
		fs:        fs,
		publisher: publisher,
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
		if tag := pkg.GetTag(); tag != "" {
			hash, err := s.meta.LookupTag(ctx, tag)
			if err != nil {
				return nil, err
			}
			pkg.Hash = hash
		}
		if !hashutil.IsStdHash(pkg.GetHash()) {
			return nil, errors.New("invalid package hash")
		}
	}

	id, err := s.meta.InsertJob(ctx, req.GetSpec())
	if err != nil {
		return nil, err
	}

	if s.publisher != nil {
		if err := s.publisher.Send(ctx); err != nil {
			return nil, err
		}
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
	jobs, err := s.meta.ListJobs(ctx, req.GetState(), req.GetLabel(), req.GetLimit(), req.GetBeforeId())
	if err != nil {
		return nil, err
	}
	return &flex.ListJobsResponse{Jobs: jobs}, nil
}

func (s *flexServer) UpdateJobLabels(ctx context.Context, req *flex.UpdateJobLabelsRequest) (*flex.UpdateJobLabelsResponse, error) {
	if err := s.meta.UpdateJobLabels(ctx, req.GetId(), req.GetAdds(), req.GetDels()); err != nil {
		return nil, err
	}
	return &flex.UpdateJobLabelsResponse{}, nil
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

	return stream.SendAndClose(&flex.InsertPackageResponse{Hash: hash})
}

func (s *flexServer) GetPackage(ctx context.Context, req *flex.GetPackageRequest) (*flex.GetPackageResponse, error) {
	if tag := req.GetTag(); tag != "" {
		hash, err := s.meta.LookupTag(ctx, tag)
		if err != nil {
			return nil, err
		}
		req.Type = &flex.GetPackageRequest_Hash{Hash: hash}
	}
	hash := req.GetHash()
	if !hashutil.IsStdHash(hash) {
		return nil, errors.New("invalid package hash")
	}

	err := s.fs.Exists(ctx, pathForPackage(hash))
	if errors.Is(err, os.ErrNotExist) {
		return nil, status.Errorf(codes.NotFound, "package not found: %s: %v", hash, err)
	}
	if err != nil {
		return nil, err
	}
	return &flex.GetPackageResponse{
		Package: &flex.Package{
			Hash: hash,
			Spec: &flex.PackageSpec{},
		},
	}, nil
}

func (s *flexServer) UpdateTag(ctx context.Context, req *flex.UpdateTagRequest) (*flex.UpdateTagResponse, error) {
	tag := req.GetTag()
	name := tag.GetName()
	if name == "" {
		return nil, errors.New("tag name is empty")
	}
	hash := tag.GetHash()
	if !hashutil.IsStdHash(hash) {
		return nil, errors.New("invalid hash")
	}
	if err := s.meta.UpdateTag(ctx, name, hash); err != nil {
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
