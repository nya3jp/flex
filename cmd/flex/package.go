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
	"encoding/json"
	"fmt"
	"os"

	"github.com/nya3jp/flex"
	"github.com/urfave/cli/v2"
)

var cmdPackage = &cli.Command{
	Name:            "package",
	Usage:           "Package-related subcommands",
	HideHelpCommand: true,
	Subcommands: []*cli.Command{
		cmdPackageCreate,
		cmdPackageTag,
		cmdPackageInfo,
		cmdPackageList,
	},
}

var cmdPackageCreate = &cli.Command{
	Name:  "create",
	Usage: "Create a package",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "tag",
			Aliases: []string{"t"},
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowSubcommandHelp(c)
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			hash, err := ensurePackage(ctx, cl, c.Args().Slice())
			if err != nil {
				return err
			}
			for _, tag := range c.StringSlice("tag") {
				if _, err := cl.UpdateTag(ctx, &flex.UpdateTagRequest{Tag: tag, Hash: hash}); err != nil {
					return err
				}
			}
			fmt.Println(hash)
			return nil
		})
	},
}

var cmdPackageTag = &cli.Command{
	Name:  "tag",
	Usage: "Tag a package",
	Action: func(c *cli.Context) error {
		if c.NArg() != 2 {
			return cli.ShowSubcommandHelp(c)
		}
		tag, hash := c.Args().Get(0), c.Args().Get(1)
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			if _, err := cl.UpdateTag(ctx, &flex.UpdateTagRequest{Tag: tag, Hash: hash}); err != nil {
				return err
			}
			fmt.Printf("%s\t%s\n", tag, hash)
			return nil
		})
	},
}

var cmdPackageInfo = &cli.Command{
	Name:  "info",
	Usage: "Show package info",
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowSubcommandHelp(c)
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			packages := make([]*flex.Package, 0) // should not be nil
			for _, name := range c.Args().Slice() {
				res, err := cl.GetPackage(ctx, &flex.GetPackageRequest{Id: packageIDFor(name)})
				if err != nil {
					return err
				}
				packages = append(packages, res.GetPackage())
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(packages)
		})
	},
}

var cmdPackageList = &cli.Command{
	Name:  "list",
	Usage: "List tagged packages",
	Action: func(c *cli.Context) error {
		if c.NArg() > 0 {
			return cli.ShowSubcommandHelp(c)
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.ListTags(ctx, &flex.ListTagsRequest{})
			if err != nil {
				return err
			}
			for _, tag := range res.GetTags() {
				fmt.Printf("%s\t%s\n", tag.GetTag(), tag.GetHash())
			}
			return nil
		})
	},
}
