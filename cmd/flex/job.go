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
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/types/known/durationpb"
)

var flagPriority = &cli.IntFlag{
	Name:  "priority",
	Value: 0,
	Usage: "Sets the priority of the new task. Higher values are higher priority.",
}

var flagFile = &cli.StringSliceFlag{
	Name:    "file",
	Aliases: []string{"f"},
	Usage:   "Adds a file or directory to the one-off package for the task. Can be repeated.",
}

var flagPackage = &cli.StringSliceFlag{
	Name:    "package",
	Aliases: []string{"p"},
	Usage:   "Adds a package to the task. Can be repeated.",
}

var flagShell = &cli.BoolFlag{
	Name:    "shell",
	Aliases: []string{"s"},
	Usage:   "Runs a command via a shell. Exactly one command argument must be specified when this flag is set.",
}

var flagTimeLimit = &cli.DurationFlag{
	Name:    "time-limit",
	Aliases: []string{"t"},
	Value:   time.Minute,
	Usage:   "Sets the time limit of the task.",
}

var flagWait = &cli.BoolFlag{
	Name:    "wait",
	Aliases: []string{"w"},
	Usage:   "Waits until the task finishes.",
}

var flagOutputs = &cli.BoolFlag{
	Name:  "outputs",
	Usage: "Prints task outputs to the console.",
}

var flagLimit = &cli.Int64Flag{
	Name:    "limit",
	Aliases: []string{"n"},
	Value:   10,
	Usage:   "Sets the maximum number of jobs returned.",
}

var flagBefore = &cli.Int64Flag{
	Name:        "before",
	Usage:       "Returns jobs whose ID is less than the specified value.",
	Value:       math.MaxInt64,
	DefaultText: "inf",
}

var flagState = &cli.StringFlag{
	Name:    "state",
	Aliases: []string{"s"},
	Usage:   "Filters jobs by state. Allowed values are: pending, running, finished.",
}

var cmdJob = &cli.Command{
	Name:            "job",
	Usage:           "Job-related subcommands.",
	HideHelpCommand: true,
	Subcommands: []*cli.Command{
		cmdJobCreate,
		cmdJobWait,
		cmdJobOutputs,
		cmdJobInfo,
		cmdJobList,
	},
}

var cmdJobCreate = &cli.Command{
	Name:      "create",
	Usage:     "Creates a new job.",
	UsageText: "flex create [command options] executable [args...]\n   flex create [command options] -s command",
	Flags: []cli.Flag{
		flagFile,
		flagPackage,
		flagShell,
		flagTimeLimit,
		flagPriority,
		flagWait,
		flagOutputs,
	},
	Action: func(c *cli.Context) error {
		priority := c.Int(flagPriority.Name)
		files := c.StringSlice(flagFile.Name)
		packages := c.StringSlice(flagPackage.Name)
		shell := c.Bool(flagShell.Name)
		timeLimit := c.Duration(flagTimeLimit.Name)
		wait := c.Bool(flagWait.Name)
		outputs := c.Bool(flagOutputs.Name)

		if outputs && !wait {
			return fmt.Errorf("--%s required for --%s", flagWait.Name, flagOutputs.Name)
		}

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
			log.Printf("Submitted job %d", id.GetIntId())

			if !wait {
				fmt.Println(id.GetIntId())
				return nil
			}

			if err := waitJob(ctx, cl, id); err != nil {
				return err
			}

			if outputs {
				if err := printJobOutputs(ctx, cl, id); err != nil {
					return err
				}
			}
			return nil
		})
	},
}

var cmdJobWait = &cli.Command{
	Name:      "wait",
	Usage:     "Waits a job.",
	ArgsUsage: "job-id",
	Flags: []cli.Flag{
		flagOutputs,
	},
	Action: func(c *cli.Context) error {
		outputs := c.Bool(flagOutputs.Name)
		if c.NArg() != 1 {
			return cli.ShowSubcommandHelp(c)
		}

		intID, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		id := &flex.JobId{IntId: intID}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			log.Printf("Waiting for job %d", id.GetIntId())
			if err := waitJob(ctx, cl, id); err != nil {
				return err
			}
			if outputs {
				if err := printJobOutputs(ctx, cl, id); err != nil {
					return err
				}
			}
			return nil
		})
	},
}

var cmdJobOutputs = &cli.Command{
	Name:      "outputs",
	Usage:     "Prints out job outputs",
	ArgsUsage: "job-id",
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			return cli.ShowSubcommandHelp(c)
		}

		intID, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		id := &flex.JobId{IntId: intID}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			if err := printJobOutputs(ctx, cl, id); err != nil {
				return err
			}
			return nil
		})
	},
}

var cmdJobInfo = &cli.Command{
	Name:      "info",
	Usage:     "Shows job info.",
	ArgsUsage: "job-id",
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			return cli.ShowSubcommandHelp(c)
		}
		intID, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		id := &flex.JobId{IntId: intID}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: id})
			if err != nil {
				return err
			}
			job := res.GetJob()
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(job)
		})
	},
}

var cmdJobList = &cli.Command{
	Name:      "list",
	Usage:     "Lists jobs.",
	ArgsUsage: "",
	Flags: []cli.Flag{
		flagLimit,
		flagBefore,
		flagState,
	},
	Action: func(c *cli.Context) error {
		limit := c.Int64(flagLimit.Name)
		before := c.Int64(flagBefore.Name)
		stateStr := c.String(flagState.Name)
		if c.NArg() > 0 {
			return cli.ShowSubcommandHelp(c)
		}

		var state flex.JobState
		switch strings.ToLower(stateStr) {
		case "":
			state = flex.JobState_UNSPECIFIED
		case "pending":
			state = flex.JobState_PENDING
		case "running":
			state = flex.JobState_RUNNING
		case "finished":
			state = flex.JobState_FINISHED
		default:
			return fmt.Errorf("unknown job state: %s", stateStr)
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.ListJobs(ctx, &flex.ListJobsRequest{
				Limit:    limit,
				BeforeId: &flex.JobId{IntId: before},
				State:    state,
			})
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

func waitJob(ctx context.Context, cl flex.FlexServiceClient, id *flex.JobId) error {
	lastState := flex.JobState_PENDING
	for {
		res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: id})
		if err != nil {
			return err
		}

		job := res.GetJob()
		state := job.GetState()
		if state != lastState {
			lastState = state
			switch state {
			case flex.JobState_PENDING:
				log.Printf("Job %d returned", id.GetIntId())
			case flex.JobState_RUNNING:
				log.Printf("Job %d running", id.GetIntId())
			case flex.JobState_FINISHED:
				result := job.GetResult()
				log.Printf("Job %d finished: %s (%v)", id.GetIntId(), result.GetMessage(), result.GetTime().AsDuration())
				return nil
			}
		}

		if err := ctxutil.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
}

func printJobOutputs(ctx context.Context, cl flex.FlexServiceClient, id *flex.JobId) error {
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
		return fmt.Errorf("failed to retrieve stderr: %v", err)
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
		return fmt.Errorf("failed to retrieve stdout: %v", err)
	}
	return nil
}
