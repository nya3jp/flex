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

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Passwords struct {
	Flex    string `json:"flex"`
	Flexlet string `json:"flexlet"`
}

type authMixin struct {
	password string
}

func newAuthMixin(password string) *authMixin {
	return &authMixin{password: password}
}

func (a *authMixin) AuthFuncOverride(ctx context.Context, _ string) (context.Context, error) {
	if a.password == "" {
		return ctx, nil
	}

	password, err := grpc_auth.AuthFromMD(ctx, "Bearer")
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	if password != a.password {
		return nil, status.Errorf(codes.PermissionDenied, "wrong auth token")
	}
	return ctx, nil
}

func defaultAuthFunc(ctx context.Context) (context.Context, error) {
	return nil, errors.New("auth not configured")
}
