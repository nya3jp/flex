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
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/urfave/cli/v2"
)

var app = &cli.App{
	Name:  "flex",
	Usage: "Flex CLI client",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "hub",
			Aliases: []string{"h"},
			Value:   "localhost:7111",
		},
	},
	HideHelpCommand: true,
	Commands: []*cli.Command{
		cmdJob,
		cmdPackage,
	},
}

func main() {
	log.SetFlags(0)
	cli.HelpFlag = &cli.BoolFlag{
		Name:  "help",
		Usage: "Show help",
	}
	if err := app.RunContext(context.Background(), os.Args); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
