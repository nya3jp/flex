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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

type GS struct {
	cl      *storage.Client
	ic      *iamcredentials.Service
	baseURL *url.URL
	email   string
}

func NewGS(ctx context.Context, baseURL string) (g *GS, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("setting up GS access: %w", err)
		}
	}()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "gs" {
		return nil, fmt.Errorf("invalid URL: expected gs://: %s", baseURL)
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		return nil, fmt.Errorf("invalid URL: should end with a slash: %s", baseURL)
	}

	creds, err := google.FindDefaultCredentials(ctx, storage.ScopeReadWrite, iamcredentials.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	cl, err := storage.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, err
	}

	ic, err := iamcredentials.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, err
	}

	email, err := getEmail(creds)
	if err != nil {
		return nil, err
	}

	return &GS{
		cl:      cl,
		ic:      ic,
		baseURL: parsed,
		email:   email,
	}, nil
}

func getEmail(creds *google.Credentials) (string, error) {
	var j struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(creds.JSON, &j); err == nil && j.ClientEmail != "" {
		return j.ClientEmail, nil
	}

	if metadata.OnGCE() {
		email, err := metadata.Email("")
		if err == nil {
			return email, nil
		}
	}
	return "", errors.New("service account email unavailable")
}

func (g *GS) Exists(ctx context.Context, path string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("checking: %w", err)
		}
	}()
	_, err = g.object(path).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return os.ErrNotExist
	}
	return err
}

func (g *GS) Put(ctx context.Context, path string, r io.ReadSeeker) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("uploading: %w", err)
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := g.object(path).NewWriter(ctx)
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Close()
}

func (g *GS) PresignedURLForGet(ctx context.Context, path string, dur time.Duration) (string, error) {
	return g.presignURL(ctx, path, http.MethodGet, dur)
}

func (g *GS) PresignedURLForPut(ctx context.Context, path string, dur time.Duration) (string, error) {
	return g.presignURL(ctx, path, http.MethodPut, dur)
}

func (g *GS) CanonicalURL(path string) string {
	obj := g.object(path)
	return fmt.Sprintf("gs://%s/%s", obj.BucketName(), obj.ObjectName())
}

func (g *GS) presignURL(ctx context.Context, path, method string, dur time.Duration) (url string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("presigning: %w", err)
		}
	}()
	obj := g.object(path)
	return storage.SignedURL(obj.BucketName(), obj.ObjectName(), &storage.SignedURLOptions{
		GoogleAccessID: g.email,
		Method:         method,
		Expires:        time.Now().Add(dur),
		SignBytes: func(bytes []byte) ([]byte, error) {
			res, err := g.ic.Projects.ServiceAccounts.SignBlob(
				"projects/-/serviceAccounts/"+g.email,
				&iamcredentials.SignBlobRequest{
					Payload: base64.StdEncoding.EncodeToString(bytes),
				}).Context(ctx).Do()
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(res.SignedBlob)
		},
	})
}

func (g *GS) object(path string) *storage.ObjectHandle {
	return g.cl.Bucket(g.baseURL.Host).Object(strings.TrimPrefix(g.baseURL.Path, "/") + path)
}
