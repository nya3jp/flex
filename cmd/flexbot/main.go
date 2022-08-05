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
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/nya3jp/flex/internal/pubsub"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	if err := func() error {
		app := &cli.App{
			Name:  "flexbot",
			Usage: "Flexbot",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "subscribe", Required: true, Usage: "PubSub subscription ID to pull job events from"},
				&cli.StringFlag{Name: "flexlet", Required: true, Usage: "Flexlet URL"},
				&cli.IntFlag{Name: "parallelism", Required: true, Usage: "Number of flexlets run in parallel"},
			},
			Action: func(c *cli.Context) error {
				ctx := c.Context
				subscriptionID := c.String("subscribe")
				flexletURL := c.String("flexlet")
				parallelism := c.Int("parallelism")

				subscriber, err := pubsub.NewSubscriber(ctx, subscriptionID, parallelism)
				if err != nil {
					return err
				}
				defer subscriber.Close()

				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				return subscriber.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
					defer msg.Nack()
					if err := func() error {
						res, err := http.Post(flexletURL, "application/octet-stream", &bytes.Buffer{})
						if err != nil {
							return err
						}
						res.Body.Close()
						if res.StatusCode/100 != 2 {
							return fmt.Errorf("http status %d", res.StatusCode)
						}
						msg.Ack()
						return nil
					}(); err != nil {
						log.Printf("ERROR: %v", err)
					}
				})
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
