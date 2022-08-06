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
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/hashutil"
)

var flagTag = &cli.StringSliceFlag{
	Name:    "tag",
	Aliases: []string{"t"},
	Usage:   "Sets a tag for the new package. Can be repeated.",
}

var cmdPackage = &cli.Command{
	Name:            "package",
	Usage:           "Package-related subcommands.",
	HideHelpCommand: true,
	Subcommands: []*cli.Command{
		cmdPackageCreate,
		cmdPackageTag,
		cmdPackageInfo,
		cmdPackageList,
	},
}

var cmdPackageCreate = &cli.Command{
	Name:      "create",
	Aliases:   []string{"new"},
	Usage:     "Creates a new package.",
	ArgsUsage: "[files...]",
	Flags: []cli.Flag{
		flagTag,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			hash, err := ensurePackage(ctx, cl, c.Args().Slice())
			if err != nil {
				return err
			}
			for _, tag := range c.StringSlice("tag") {
				if _, err := cl.UpdateTag(ctx, &flex.UpdateTagRequest{Tag: &flex.Tag{Name: tag, Hash: hash}}); err != nil {
					return err
				}
			}
			fmt.Println(hash)
			return nil
		})
	},
}

var cmdPackageTag = &cli.Command{
	Name:      "tag",
	Usage:     "Sets a tag to a package.",
	ArgsUsage: "tag hash",
	Flags: []cli.Flag{
		flagJSON,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 2 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		tag, hash := c.Args().Get(0), c.Args().Get(1)
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			tag := &flex.Tag{Name: tag, Hash: hash}
			if _, err := cl.UpdateTag(ctx, &flex.UpdateTagRequest{Tag: tag}); err != nil {
				return err
			}
			newOutputFormatter(c).Tag(tag)
			return nil
		})
	},
}

var cmdPackageInfo = &cli.Command{
	Name:      "info",
	Aliases:   []string{"get"},
	Usage:     "Shows package info.",
	ArgsUsage: "{hash|tag}",
	Flags: []cli.Flag{
		flagJSON,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		name := c.Args().Get(0)
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			var req *flex.GetPackageRequest
			if hashutil.IsStdHash(name) {
				req = &flex.GetPackageRequest{Type: &flex.GetPackageRequest_Hash{Hash: name}}
			} else {
				req = &flex.GetPackageRequest{Type: &flex.GetPackageRequest_Tag{Tag: name}}
			}
			res, err := cl.GetPackage(ctx, req)
			if err != nil {
				return err
			}
			newOutputFormatter(c).Package(res.GetPackage())
			return nil
		})
	},
}

var cmdPackageList = &cli.Command{
	Name:      "list",
	Aliases:   []string{"ls"},
	Usage:     "Lists tagged packages.",
	ArgsUsage: "",
	Flags: []cli.Flag{
		flagJSON,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() > 0 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.ListTags(ctx, &flex.ListTagsRequest{})
			if err != nil {
				return err
			}
			newOutputFormatter(c).Tags(res.GetTags())
			return nil
		})
	},
}
