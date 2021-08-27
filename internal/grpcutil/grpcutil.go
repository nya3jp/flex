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

package grpcutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func DialContext(ctx context.Context, serverURL string, password string) (*grpc.ClientConn, error) {
	host, opts, err := parseServerURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gRPC server URL: %w", err)
	}
	if password != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&passwordCredentials{password}))
	}
	return grpc.DialContext(ctx, host, opts...)
}

func parseServerURL(serverURL string) (host string, opts []grpc.DialOption, err error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", nil, err
	}
	if strings.Trim(parsed.Path, "/") != "" {
		return "", nil, errors.New("path must be empty")
	}

	switch parsed.Scheme {
	case "http":
		opts := []grpc.DialOption{grpc.WithAuthority(parsed.Host), grpc.WithInsecure()}
		return hostWithDefaultPort(parsed.Host, 80), opts, nil
	case "https":
		pool, err := x509.SystemCertPool()
		if err != nil {
			return "", nil, err
		}
		creds := credentials.NewTLS(&tls.Config{RootCAs: pool})
		opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
		return hostWithDefaultPort(parsed.Host, 443), opts, nil
	default:
		return "", nil, errors.New("scheme must be https or http")
	}
}

func hostWithDefaultPort(host string, defaultPort int) string {
	if _, _, err := net.SplitHostPort(host); err != nil {
		return fmt.Sprintf("%s:%d", host, defaultPort)
	}
	return host
}

type passwordCredentials struct {
	password string
}

func (c *passwordCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + c.password}, nil
}

func (c *passwordCredentials) RequireTransportSecurity() bool {
	return false
}
