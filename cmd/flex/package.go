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
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

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
		cmdPackageResolve,
		cmdPackageList,
		cmdPackageInspect,
		cmdPackageDownload,
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

var cmdPackageResolve = &cli.Command{
	Name:      "resolve",
	Usage:     "Resolves a tag to a hash.",
	ArgsUsage: "tag",
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

var cmdPackageInspect = &cli.Command{
	Name:      "inspect",
	Usage:     "Prints contents of a package.",
	ArgsUsage: "{hash|tag}",
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		name := c.Args().Get(0)

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			var req *flex.FetchPackageRequest
			if hashutil.IsStdHash(name) {
				req = &flex.FetchPackageRequest{Type: &flex.FetchPackageRequest_Hash{Hash: name}}
			} else {
				req = &flex.FetchPackageRequest{Type: &flex.FetchPackageRequest_Tag{Tag: name}}
			}
			res, err := cl.FetchPackage(ctx, req)
			if err != nil {
				return err
			}

			url := res.GetLocation().GetPresignedUrl()
			r, err := http.Get(url)
			if err != nil {
				return err
			}
			defer r.Body.Close()

			if r.StatusCode != http.StatusOK {
				return fmt.Errorf("http status %d", r.StatusCode)
			}

			cmd := exec.Command("tar", "tvz")
			cmd.Stdin = r.Body
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return err
			}
			return nil
		})
	},
}

var flagPackageOutput = &cli.StringFlag{
	Name:    "output",
	Aliases: []string{"o"},
	Usage:   "File path to download a package to (default: ./<name>.tar.gz)",
}

var cmdPackageDownload = &cli.Command{
	Name:      "download",
	Usage:     "Downloads a package.",
	ArgsUsage: "{hash|tag}",
	Flags: []cli.Flag{
		flagPackageOutput,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		name := c.Args().Get(0)
		outputPath := c.String(flagPackage.Name)
		if outputPath == "" {
			outputPath = fmt.Sprintf("./%s.tar.gz", name)
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			var req *flex.FetchPackageRequest
			if hashutil.IsStdHash(name) {
				req = &flex.FetchPackageRequest{Type: &flex.FetchPackageRequest_Hash{Hash: name}}
			} else {
				req = &flex.FetchPackageRequest{Type: &flex.FetchPackageRequest_Tag{Tag: name}}
			}
			res, err := cl.FetchPackage(ctx, req)
			if err != nil {
				return err
			}

			url := res.GetLocation().GetPresignedUrl()
			r, err := http.Get(url)
			if err != nil {
				return err
			}
			defer r.Body.Close()

			if r.StatusCode != http.StatusOK {
				return fmt.Errorf("http status %d", r.StatusCode)
			}

			f, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(f, r.Body); err != nil {
				return err
			}

			if err := f.Close(); err != nil {
				return err
			}

			log.Printf("Saved to %s", outputPath)
			return nil
		})
	},
}
