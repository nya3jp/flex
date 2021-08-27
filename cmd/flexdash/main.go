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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/grpcutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	if err := func() error {
		defaultPort := 8080
		if port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
			defaultPort = port
		}

		app := &cli.App{
			Name:  "flexdash",
			Usage: "Flex Dashboard",
			Flags: []cli.Flag{
				&cli.IntFlag{Name: "port", Value: defaultPort, Usage: "TCP port to listen on"},
				&cli.StringFlag{Name: "hub", Required: true, Usage: "Flexhub address"},
				&cli.BoolFlag{Name: "insecure", Usage: "Use insecure connections to Flexhub servers"},
				&cli.StringFlag{Name: "password", Usage: "Sets a Flex service password"},
			},
			Action: func(c *cli.Context) error {
				port := c.Int("port")
				hubAddr := c.String("hub")
				insecure := c.Bool("insecure")
				password := c.String("password")

				ctx := c.Context

				cc, err := grpcutil.DialContext(ctx, hubAddr, insecure, password)
				if err != nil {
					return err
				}
				cl := flex.NewFlexServiceClient(cc)

				server := http.Server{
					Addr:        fmt.Sprintf("0.0.0.0:%d", port),
					Handler:     newRouter(cl),
					BaseContext: func(net.Listener) context.Context { return ctx },
				}
				go func() {
					<-ctx.Done()
					log.Print("INFO: Shutting down the server")
					server.Close()
				}()
				log.Printf("INFO: Listening at %s", server.Addr)
				return server.ListenAndServe()
			},
		}
		return app.RunContext(ctx, os.Args)
	}(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
