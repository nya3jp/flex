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
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/hashutil"
)

var flagPriority = &cli.IntFlag{
	Name:  "priority",
	Value: 0,
	Usage: "Sets the priority of the new job. Higher values are higher priority.",
}

var flagFile = &cli.StringSliceFlag{
	Name:    "file",
	Aliases: []string{"f"},
	Usage:   "Adds a file or directory to the one-off package for the job. Can be repeated.",
}

var flagPackage = &cli.StringSliceFlag{
	Name:    "package",
	Aliases: []string{"p"},
	Usage:   "Adds a package to the job. Can be repeated.",
}

var flagAddLabel = &cli.StringSliceFlag{
	Name:    "label",
	Aliases: []string{"l"},
	Usage:   "Adds a label to the job. Can be repeated.",
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
	Usage:   "Sets the time limit of the job.",
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

var flagLabel = &cli.StringFlag{
	Name:    "label",
	Aliases: []string{"l"},
	Usage:   "Filters jobs by label.",
}

var flagAdd = &cli.StringSliceFlag{
	Name:    "add",
	Aliases: []string{"a"},
	Usage:   "Adds a label.",
}

var flagDelete = &cli.StringSliceFlag{
	Name:    "delete",
	Aliases: []string{"del", "d"},
	Usage:   "Deletes a label.",
}

var jobCreateFlags = []cli.Flag{
	flagFile,
	flagPackage,
	flagShell,
	flagTimeLimit,
	flagPriority,
	flagAddLabel,
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
		cmdJobLabel,
	},
}

var cmdRun = &cli.Command{
	Name:      "run",
	Usage:     "Runs a job.",
	UsageText: "flex run [command options] executable [args...]\n   flex run [command options] -s command",
	Flags:     jobCreateFlags,
	Action: func(c *cli.Context) error {
		args, err := makeArgs(c)
		if err != nil {
			return err
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			id, err := submitJob(ctx, cl, c, args)
			if err != nil {
				return err
			}
			if err := waitJob(ctx, cl, id); err != nil {
				return err
			}
			if err := printJobOutputs(ctx, cl, id); err != nil {
				return err
			}
			return nil
		})
	},
}

var cmdJobCreate = &cli.Command{
	Name:      "create",
	Aliases:   []string{"new"},
	Usage:     "Creates a new job.",
	UsageText: "flex job create [command options] executable [args...]\n   flex job create [command options] -s command",
	Flags:     jobCreateFlags,
	Action: func(c *cli.Context) error {
		args, err := makeArgs(c)
		if err != nil {
			return err
		}
		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			id, err := submitJob(ctx, cl, c, args)
			if err != nil {
				return err
			}
			fmt.Println(id)
			return nil
		})
	},
}

var cmdJobWait = &cli.Command{
	Name:      "wait",
	Usage:     "Waits a job.",
	ArgsUsage: "job-id",
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}

		id, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			log.Printf("Waiting for job %d", id)
			if err := waitJob(ctx, cl, id); err != nil {
				return err
			}
			return nil
		})
	},
}

var cmdJobOutputs = &cli.Command{
	Name:      "outputs",
	Aliases:   []string{"cat"},
	Usage:     "Prints out job outputs",
	ArgsUsage: "job-id",
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}

		id, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

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
	Aliases:   []string{"get"},
	Usage:     "Shows job info.",
	ArgsUsage: "job-id",
	Flags: []cli.Flag{
		flagJSON,
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		id, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: id})
			if err != nil {
				return err
			}
			job := res.GetJob()
			newOutputFormatter(c).JobStatus(job)
			return nil
		})
	},
}

var cmdJobList = &cli.Command{
	Name:      "list",
	Aliases:   []string{"ls"},
	Usage:     "Lists jobs.",
	ArgsUsage: "",
	Flags: []cli.Flag{
		flagLimit,
		flagBefore,
		flagState,
		flagLabel,
		flagJSON,
	},
	Action: func(c *cli.Context) error {
		limit := c.Int64(flagLimit.Name)
		beforeID := c.Int64(flagBefore.Name)
		stateStr := c.String(flagState.Name)
		label := c.String(flagLabel.Name)
		if c.NArg() > 0 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
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
				BeforeId: beforeID,
				State:    state,
				Label:    label,
			})
			if err != nil {
				return err
			}
			jobs := res.GetJobs()
			newOutputFormatter(c).JobStatuses(jobs)
			return nil
		})
	},
}

var cmdJobLabel = &cli.Command{
	Name:      "label",
	Usage:     "Updates job labels.",
	ArgsUsage: "job-id",
	Flags: []cli.Flag{
		flagAdd,
		flagDelete,
	},
	Action: func(c *cli.Context) error {
		adds := c.StringSlice(flagAdd.Name)
		dels := c.StringSlice(flagDelete.Name)
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		id, err := strconv.ParseInt(c.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

		return runCmd(c, func(ctx context.Context, cl flex.FlexServiceClient) error {
			req := &flex.UpdateJobLabelsRequest{
				Id:   id,
				Adds: adds,
				Dels: dels,
			}
			if _, err := cl.UpdateJobLabels(ctx, req); err != nil {
				return err
			}
			return nil
		})
	},
}

func makeArgs(c *cli.Context) ([]string, error) {
	if c.Bool(flagShell.Name) {
		if c.NArg() != 1 {
			cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
		}
		return []string{"sh", "-e", "-c", c.Args().Get(0)}, nil
	}
	if c.NArg() == 0 {
		cli.ShowSubcommandHelpAndExit(c, exitCodeHelp)
	}
	return c.Args().Slice(), nil
}

func submitJob(ctx context.Context, cl flex.FlexServiceClient, c *cli.Context, args []string) (int64, error) {
	priority := c.Int(flagPriority.Name)
	files := c.StringSlice(flagFile.Name)
	packages := c.StringSlice(flagPackage.Name)
	timeLimit := c.Duration(flagTimeLimit.Name)
	labels := c.StringSlice(flagAddLabel.Name)

	if len(files) > 0 {
		hash, err := ensurePackage(ctx, cl, files)
		if err != nil {
			return 0, err
		}
		packages = append(packages, hash)
	}

	var pkgs []*flex.JobPackage
	for _, p := range packages {
		var pkg *flex.JobPackage
		if hashutil.IsStdHash(p) {
			pkg = &flex.JobPackage{Hash: p}
		} else {
			pkg = &flex.JobPackage{Tag: p}
		}
		pkgs = append(pkgs, pkg)
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
		Annotations: &flex.JobAnnotations{
			Labels: labels,
		},
	}
	res, err := cl.SubmitJob(ctx, &flex.SubmitJobRequest{Spec: spec})
	if err != nil {
		return 0, err
	}

	log.Printf("Submitted job %d", res.GetId())
	return res.GetId(), nil
}

func waitJob(ctx context.Context, cl flex.FlexServiceClient, id int64) error {
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
				log.Printf("Job %d returned", id)
			case flex.JobState_RUNNING:
				log.Printf("Job %d running", id)
			case flex.JobState_FINISHED:
				result := job.GetResult()
				log.Printf("Job %d finished: %s (%v)", id, result.GetMessage(), result.GetTime().AsDuration())
				return nil
			}
		}

		if err := ctxutil.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
}

func printJobOutputs(ctx context.Context, cl flex.FlexServiceClient, id int64) error {
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
