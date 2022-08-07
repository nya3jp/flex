// Copyright 2022 Google LLC
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
	"log"
	"os"
	"os/signal"

	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/grpcutil"
)

func run(ctx context.Context, cl flex.FlexServiceClient, args []string, jobs int) error {
	var jobIds []int64

	for i := 0; i < jobs; i++ {
		res, err := cl.SubmitJob(ctx, &flex.SubmitJobRequest{
			Spec: &flex.JobSpec{
				Command: &flex.JobCommand{
					Args: args,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to submit a job: %w", err)
		}
		jobIds = append(jobIds, res.GetId())
	}

	log.Printf("Submitted %d jobs; waiting for their results", jobs)

	for _, jobId := range jobIds {
		for {
			res, err := cl.GetJob(ctx, &flex.GetJobRequest{Id: jobId})
			if err != nil {
				return fmt.Errorf("failed to poll job %d: %w", jobId, err)
			}
			state := res.GetJob().GetState()
			if state != flex.JobState_FINISHED {
				continue
			}
			if res.GetJob().GetResult().GetExitCode() != 0 {
				return fmt.Errorf("job %d failed: %s", jobId, res.GetJob().GetResult().GetMessage())
			}
			break
		}
	}

	log.Print("All jobs finished successfully")

	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	if err := func() error {
		app := &cli.App{
			Name: "loadtest",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub URL"},
				&cli.StringFlag{Name: "password", Usage: "Sets a Flex service password"},
				&cli.IntFlag{Name: "jobs", Required: true, Usage: "Number of jobs to submit"},
			},
			Action: func(c *cli.Context) error {
				hubURL := c.String("hub")
				password := c.String("password")
				jobs := c.Int("jobs")
				args := c.Args().Slice()

				cc, err := grpcutil.DialContext(ctx, hubURL, password)
				if err != nil {
					return err
				}
				cl := flex.NewFlexServiceClient(cc)

				return run(ctx, cl, args, jobs)
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
