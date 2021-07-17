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
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/types/known/durationpb"
)

var cmdJob = &cli.Command{
	Name:            "job",
	Usage:           "Job-related subcommands",
	HideHelpCommand: true,
	Subcommands: []*cli.Command{
		cmdJobCreate,
		cmdJobInfo,
		cmdJobList,
	},
}

var cmdJobCreate = &cli.Command{
	Name:  "create",
	Usage: "Create a job",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "priority",
			Value: 0,
		},
		&cli.StringSliceFlag{
			Name:    "file",
			Aliases: []string{"f"},
		},
		&cli.StringSliceFlag{
			Name:    "package",
			Aliases: []string{"p"},
		},
		&cli.BoolFlag{
			Name:    "shell",
			Aliases: []string{"s"},
		},
		&cli.DurationFlag{
			Name:    "time",
			Aliases: []string{"t"},
			Value:   time.Minute,
		},
		&cli.BoolFlag{
			Name:    "wait",
			Aliases: []string{"w"},
		},
	},
	Action: func(c *cli.Context) error {
		priority := c.Int("priority")
		files := c.StringSlice("file")
		packages := c.StringSlice("package")
		shell := c.Bool("shell")
		timeLimit := c.Duration("time")
		wait := c.Bool("wait")
		var args []string
		if shell {
			if c.NArg() != 1 {
				return cli.ShowSubcommandHelp(c)
			}
			args = []string{"sh", "-c", c.Args().Get(0)}
		} else {
			if c.NArg() == 0 {
				return cli.ShowSubcommandHelp(c)
			}
			args = c.Args().Slice()
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			if len(files) > 0 {
				hash, err := ensurePackage(ctx, cl, files)
				if err != nil {
					return err
				}
				packages = append(packages, hash)
			}

			var pkgs []*flex.JobPackage
			for _, p := range packages {
				pkgs = append(pkgs, &flex.JobPackage{Id: packageIDFor(p)})
			}

			spec := &flex.JobSpec{
				Command: &flex.JobCommand{
					Args: args,
				},
				Inputs: &flex.JobInputs{
					Packages: pkgs,
				},
				Limits: &flex.JobLimits{
					Time: durationpb.New(timeLimit),
				},
				Constraints: &flex.JobConstraints{
					Priority: int32(priority),
				},
			}
			res, err := cl.SubmitJob(ctx, &flex.SubmitJobRequest{Spec: spec})
			if err != nil {
				return err
			}

			id := res.GetId()
			if !wait {
				fmt.Println(id.GetIntId())
				return nil
			}

			log.Printf("Submitted job %d", id.GetIntId())

			started := false
			for {
				res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: id})
				if err != nil {
					return err
				}

				job := res.GetJob()
				state := job.GetState()
				if state != flex.JobState_PENDING && !started {
					log.Print("Job started")
					started = true
				}
				if state == flex.JobState_FINISHED {
					result := job.GetResult()
					log.Printf("Job finished: %s (%v)", result.GetMessage(), result.GetTime().AsDuration())
					break
				}

				if err := ctxutil.Sleep(ctx, time.Second); err != nil {
					return err
				}
			}

			if err := func() error {
				jo, err := cl.GetJobOutput(ctx, &flex.GetJobOutputRequest{Id: id, Type: flex.GetJobOutputRequest_STDERR})
				if err != nil {
					return err
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, jo.GetLocation().GetPresignedUrl(), nil)
				if err != nil {
					return err
				}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer res.Body.Close()
				_, err = io.Copy(os.Stderr, res.Body)
				return err
			}(); err != nil {
				log.Printf("WARNING: Failed to retrieve stderr: %v", err)
			}

			if err := func() error {
				jo, err := cl.GetJobOutput(ctx, &flex.GetJobOutputRequest{Id: id, Type: flex.GetJobOutputRequest_STDOUT})
				if err != nil {
					return err
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, jo.GetLocation().GetPresignedUrl(), nil)
				if err != nil {
					return err
				}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer res.Body.Close()
				_, err = io.Copy(os.Stdout, res.Body)
				return err
			}(); err != nil {
				log.Printf("WARNING: Failed to retrieve stdout: %v", err)
			}
			return nil
		})
	},
}

var cmdJobInfo = &cli.Command{
	Name:  "info",
	Usage: "Show job info",
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowSubcommandHelp(c)
		}
		var ids []int64
		for _, s := range c.Args().Slice() {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			jobs := make([]*flex.JobStatus, 0) // should not be nil
			for _, id := range ids {
				res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: &flex.JobId{IntId: id}})
				if err != nil {
					return err
				}
				jobs = append(jobs, res.GetJob())
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(jobs)
		})
	},
}

var cmdJobList = &cli.Command{
	Name:  "list",
	Usage: "List jobs",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:    "limit",
			Aliases: []string{"n"},
			Value:   10,
		},
	},
	Action: func(c *cli.Context) error {
		limit := c.Int64("limit")
		if c.NArg() > 0 {
			return cli.ShowSubcommandHelp(c)
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.ListJobs(ctx, &flex.ListJobsRequest{Limit: limit})
			if err != nil {
				return err
			}
			jobs := res.GetJobs()
			if jobs == nil {
				jobs = make([]*flex.JobStatus, 0)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(jobs)
		})
	},
}
