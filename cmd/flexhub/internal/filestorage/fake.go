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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Anonymous struct {
	baseURL *url.URL
}

func NewAnonymous(baseURL string) (a *Anonymous, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("setting up anonymous access: %w", err)
		}
	}()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("invalid URL: expected http://: %s", baseURL)
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		return nil, fmt.Errorf("invalid URL: should end with a slash: %s", baseURL)
	}

	return &Anonymous{
		baseURL: parsed,
	}, nil
}

func (g *Anonymous) Exists(ctx context.Context, path string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("checking: %w", err)
		}
	}()

	res, err := http.DefaultClient.Do(g.request(http.MethodHead, path, nil).WithContext(ctx))
	if err != nil {
		return err
	}
	res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return os.ErrNotExist
	}
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("http: %s", res.Status)
	}
	return nil
}

func (g *Anonymous) Put(ctx context.Context, path string, r io.ReadSeeker) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("uploading: %w", err)
		}
	}()

	res, err := http.DefaultClient.Do(g.request(http.MethodPut, path, r).WithContext(ctx))
	if err != nil {
		return err
	}
	res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return os.ErrNotExist
	}
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("http: %s", res.Status)
	}
	return nil
}

func (g *Anonymous) PresignedURLForGet(ctx context.Context, path string, dur time.Duration) (url string, err error) {
	return g.CanonicalURL(path), nil
}

func (g *Anonymous) PresignedURLForPut(ctx context.Context, path string, dur time.Duration) (string, error) {
	return g.CanonicalURL(path), nil
}

func (g *Anonymous) CanonicalURL(path string) string {
	return g.request(http.MethodGet, path, nil).URL.String()
}

func (g *Anonymous) request(method, path string, body io.Reader) *http.Request {
	if body != nil {
		body = struct{ io.Reader }{body}
	}

	u, err := g.baseURL.Parse(path)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		panic(err)
	}
	return req
}
