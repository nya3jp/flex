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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func DialContext(ctx context.Context, addr string, insecure bool) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if insecure {
		opts = append(opts, grpc.WithAuthority(addr), grpc.WithInsecure())
	} else {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		creds := credentials.NewTLS(&tls.Config{RootCAs: pool})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	return grpc.DialContext(ctx, addr, opts...)
}
