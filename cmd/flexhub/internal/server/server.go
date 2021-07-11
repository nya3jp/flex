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
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/nya3jp/flex/cmd/flexhub/internal/taskqueue"
	"github.com/nya3jp/flex/flexpb"
	"google.golang.org/grpc"
)

func processTask(ctx context.Context, tq *taskqueue.TaskQueue, gcs *storage.Client, artifactsURL *url.URL, cl flexpb.WorkerClient, worker *flexpb.WorkerInfo) (err error) {
	task, err := tq.Take(ctx, worker.GetName())
	if err != nil {
		return err
	}
	taskID := task.GetId().GetId()
	defer func() {
		if err != nil {
			tq.Reset(ctx, taskID)
		}
	}()

	log.Printf("[%s] Start task %d", worker.GetName(), taskID)
	defer func() {
		if err != nil {
			log.Printf("[%s] Failed task %d: %v", worker.GetName(), taskID, err)
		} else {
			log.Printf("[%s] Succeeded task %d", worker.GetName(), taskID)
		}
	}()

	// Check availability of the worker first.
	if _, err := cl.GetWorkerInfo(ctx, &flexpb.GetWorkerInfoRequest{}); err != nil {
		return err
	}

	bucket := gcs.Bucket(artifactsURL.Host)
	basePath := fmt.Sprintf("%stasks/%d/", strings.TrimPrefix(artifactsURL.Path, "/"), taskID)
	stdout := bucket.Object(basePath + "stdout.txt").NewWriter(ctx)
	defer stdout.Close()
	stderr := bucket.Object(basePath + "stderr.txt").NewWriter(ctx)
	defer stderr.Close()

	stream, err := cl.RunTask(ctx, &flexpb.RunTaskRequest{Task: task})
	if err != nil {
		return err
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}

		switch res := res.GetType().(type) {
		case *flexpb.RunTaskResponse_Output:
			if data := res.Output.GetStdout(); len(data) > 0 {
				stdout.Write(data)
			}
			if data := res.Output.GetStderr(); len(data) > 0 {
				stderr.Write(data)
			}
		case *flexpb.RunTaskResponse_Result:
			if err := tq.Finish(ctx, taskID, res.Result); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unknown RunTaskResponse type: %T", res)
		}
	}
}

func serveWorker(ctx context.Context, conn net.Conn, tq *taskqueue.TaskQueue, gcs *storage.Client, artifactsURL *url.URL) error {
	cc, err := grpc.DialContext(
		ctx,
		"unused",
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return conn, nil }))
	if err != nil {
		return err
	}
	defer cc.Close()

	cl := flexpb.NewWorkerClient(cc)

	res, err := cl.GetWorkerInfo(ctx, &flexpb.GetWorkerInfoRequest{})
	if err != nil {
		return err
	}

	worker := res.GetInfo()

	log.Printf("[%s] Up", worker.GetName())
	defer log.Printf("[%s] Down", worker.GetName())

	for {
		err := processTask(ctx, tq, gcs, artifactsURL, cl, worker)
		if err == taskqueue.ErrNoPendingTask {
			time.Sleep(time.Second)
			continue
		}
		if err != nil {
			return err
		}
	}
}

func Run(ctx context.Context, port int, tq *taskqueue.TaskQueue, gcs *storage.Client, artifactsURL *url.URL) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}
	defer lis.Close()

	log.Printf("Serving at 0.0.0.0:%d", port)

	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}

		go func() {
			if err := serveWorker(ctx, conn, tq, gcs, artifactsURL); err != nil {
				log.Printf("ERROR: Worker: %v", err)
			}
		}()
	}
}
