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
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type uniFS struct {
	client func() (*storage.Client, error)
}

func newUniFS(ctx context.Context, opts ...option.ClientOption) *uniFS {
	return &uniFS{client: newLazyStorageClient(ctx, opts...)}
}

type uniFSReader struct {
	r io.ReadCloser
}

func (r *uniFSReader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

func (r *uniFSReader) Close() error {
	return r.r.Close()
}

var _ io.ReadCloser = &uniFSReader{}

func (fs *uniFS) OpenForRead(ctx context.Context, url string) (*uniFSReader, error) {
	bucket, path, err := parseGSURL(url)
	if err != nil {
		r, err := os.Open(url)
		if err != nil {
			return nil, err
		}
		return &uniFSReader{r}, nil
	}

	cl, err := fs.client()
	if err != nil {
		return nil, err
	}
	r, err := cl.Bucket(bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return &uniFSReader{r}, nil
}

type uniFSWriter struct {
	w io.WriteCloser
}

func (w *uniFSWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

func (w *uniFSWriter) Close() error {
	return w.w.Close()
}

var _ io.WriteCloser = &uniFSWriter{}

func (fs *uniFS) OpenForWrite(ctx context.Context, url string) (*uniFSWriter, error) {
	bucket, path, err := parseGSURL(url)
	if err != nil {
		if err := os.MkdirAll(filepath.Dir(url), 0700); err != nil {
			return nil, err
		}
		// TODO: Delete the file if ctx is canceled.
		w, err := os.Create(url)
		if err != nil {
			return nil, err
		}
		return &uniFSWriter{w}, nil
	}

	cl, err := fs.client()
	if err != nil {
		return nil, err
	}
	w := cl.Bucket(bucket).Object(path).NewWriter(ctx)
	// TODO: Auto-detect MIME type.
	w.ContentType = "application/octet-stream"
	w.ContentEncoding = "gzip"
	return &uniFSWriter{newGzipWrapper(w)}, nil
}

type gzipWrapper struct {
	w io.WriteCloser
	g *gzip.Writer
}

func (g *gzipWrapper) Write(p []byte) (n int, err error) {
	return g.g.Write(p)
}

func (g *gzipWrapper) Close() error {
	gerr := g.g.Close()
	werr := g.w.Close()
	if gerr != nil {
		return gerr
	}
	return werr
}

func newGzipWrapper(w io.WriteCloser) *gzipWrapper {
	return &gzipWrapper{w, gzip.NewWriter(w)}
}

func newLazyStorageClient(ctx context.Context, opts ...option.ClientOption) func() (*storage.Client, error) {
	var once sync.Once
	var cl *storage.Client
	var err error
	return func() (*storage.Client, error) {
		once.Do(func() {
			cl, err = storage.NewClient(ctx, opts...)
		})
		return cl, err
	}
}

func parseGSURL(rawURL string) (bucket, path string, err error) {
	p, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	if p.Scheme != "gs" {
		return "", "", errors.New("not a GS URL")
	}
	return p.Host, strings.TrimPrefix(p.Path, "/"), nil
}
