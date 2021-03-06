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

package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/cmd/flex/internal/detar"
	"github.com/nya3jp/flex/internal/grpcutil"
	"github.com/nya3jp/flex/internal/hashutil"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var flagHub = &cli.StringFlag{
	Name:    "hub",
	Aliases: []string{"h"},
	Value:   "localhost:7111",
	Usage:   "Specifies a flexhub address in host:port format.",
}

var flagInsecure = &cli.BoolFlag{
	Name:    "insecure",
	Aliases: []string{"I"},
	Usage:   "Allows insecure connections to flexhub servers.",
}

func runCmd(c *cli.Context, f func(ctx context.Context, cl flex.FlexServiceClient) error) error {
	ctx := c.Context
	hubAddr := c.String(flagHub.Name)
	insecure := c.Bool(flagInsecure.Name)

	cc, err := grpcutil.DialContext(ctx, hubAddr, insecure)
	if err != nil {
		return err
	}
	defer cc.Close()

	cl := flex.NewFlexServiceClient(cc)
	return f(ctx, cl)
}

func packageIDFor(name string) *flex.PackageId {
	if hashutil.IsStdHash(name) {
		return &flex.PackageId{Hash: name}
	}
	return &flex.PackageId{Tag: name}
}

func ensurePackage(ctx context.Context, cl flex.FlexServiceClient, names []string) (string, error) {
	if len(names) == 0 {
		return "", errors.New("no file to package")
	}

	log.Print("Creating a new package")

	f, err := os.CreateTemp("", "flex.")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	hasher := hashutil.NewTeeHasher(f, hashutil.NewStdHash())
	if err := detar.Create(ctx, hasher, names...); err != nil {
		return "", err
	}

	hash := hasher.SumString()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	log.Printf("Hash: %s", hash)

	_, err = cl.GetPackage(ctx, &flex.GetPackageRequest{Id: packageIDFor(hash)})
	if err == nil {
		log.Print("Skipped uploading a package: already exists")
		return hash, nil
	}
	if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
		return "", err
	}

	log.Print("Uploading a package")

	stream, err := cl.InsertPackage(ctx)
	if err != nil {
		return "", err
	}
	defer stream.CloseSend()

	if err := stream.Send(&flex.InsertPackageRequest{Type: &flex.InsertPackageRequest_Spec{Spec: &flex.PackageSpec{}}}); err != nil {
		return "", err
	}

	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if err := stream.Send(&flex.InsertPackageRequest{Type: &flex.InsertPackageRequest_Data{Data: buf[:n]}}); err != nil {
			return "", err
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return res.GetId().GetHash(), nil
}
