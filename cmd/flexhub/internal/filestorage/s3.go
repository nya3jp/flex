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

package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type S3 struct {
	cl      *s3.Client
	baseURL *url.URL
}

func NewS3(ctx context.Context, baseURL string) (s *S3, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("setting up S3 access: %w", err)
		}
	}()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "s3" {
		return nil, fmt.Errorf("invalid URL: expected s3://: %s", baseURL)
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		return nil, fmt.Errorf("invalid URL: should end with a slash: %s", baseURL)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	cl := s3.NewFromConfig(cfg)
	return &S3{
		cl:      cl,
		baseURL: parsed,
	}, nil
}

func (s *S3) Exists(ctx context.Context, path string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("checking: %w", err)
		}
	}()
	fullPath := s.fullPath(path)
	_, err = s.cl.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.baseURL.Host,
		Key:    &fullPath,
	})
	var rerr *smithyhttp.ResponseError
	if errors.As(err, &rerr) && rerr.HTTPStatusCode() == http.StatusNotFound {
		return os.ErrNotExist
	}
	return err
}

func (s *S3) Put(ctx context.Context, path string, r io.ReadSeeker) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("uploading: %w", err)
		}
	}()
	fullPath := s.fullPath(path)
	_, err = s.cl.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.baseURL.Host,
		Key:    &fullPath,
		Body:   r,
	})
	return err
}

func (s *S3) PresignedURLForGet(ctx context.Context, path string, dur time.Duration) (url string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("presigning: %w", err)
		}
	}()
	fullPath := s.fullPath(path)
	ps := s3.NewPresignClient(s.cl)
	req, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.baseURL.Host,
		Key:    &fullPath,
	}, s3.WithPresignExpires(dur))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *S3) PresignedURLForPut(ctx context.Context, path string, dur time.Duration) (url string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("presigning: %w", err)
		}
	}()
	fullPath := s.fullPath(path)
	ps := s3.NewPresignClient(s.cl)
	req, err := ps.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.baseURL.Host,
		Key:    &fullPath,
	}, s3.WithPresignExpires(dur))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *S3) CanonicalURL(path string) string {
	return fmt.Sprintf("s3://%s/%s", s.baseURL.Host, s.fullPath(path))
}

func (s *S3) fullPath(path string) string {
	return strings.TrimPrefix(s.baseURL.Path, "/") + path
}
