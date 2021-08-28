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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var anonymousAllowedMethods = map[string]struct{}{
	"/flex.FlexService/GetJob":       {},
	"/flex.FlexService/GetJobOutput": {},
	"/flex.FlexService/GetPackage":   {},
	"/flex.FlexService/GetStats":     {},
	"/flex.FlexService/ListFlexlets": {},
	"/flex.FlexService/ListJobs":     {},
	"/flex.FlexService/ListTags":     {},
}

func makeAuthOptions(password string) []grpc.ServerOption {
	authenticate := func(ctx context.Context, method string) error {
		if password == "" {
			return nil
		}
		if _, ok := anonymousAllowedMethods[method]; ok {
			return nil
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return errors.New("metadata unavailable")
		}
		auths := md.Get("authorization")
		if len(auths) == 0 {
			return status.Error(codes.Unauthenticated, "authentication required")
		}
		if auths[0] != "Bearer "+password {
			return status.Error(codes.PermissionDenied, "wrong password")
		}
		return nil
	}

	return []grpc.ServerOption{
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (res interface{}, err error) {
			if err := authenticate(ctx, info.FullMethod); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if err := authenticate(stream.Context(), info.FullMethod); err != nil {
				return err
			}
			return handler(srv, stream)
		}),
	}
}
