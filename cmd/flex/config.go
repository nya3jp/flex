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
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
	"gopkg.in/ini.v1"
)

type Config struct {
	HubURL   string
	Password string
}

var configPath = func() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(homeDir, ".config", "flex", "config.ini")
}()

var config = func() Config {
	cfg, err := loadConfig()
	if err != nil {
		cfg = defaultConfig()
	}
	return cfg
}()

func defaultConfig() Config {
	return Config{
		HubURL:   "http://localhost:7111",
		Password: "",
	}
}

func loadConfig() (Config, error) {
	f, err := ini.Load(configPath)
	if err != nil {
		return defaultConfig(), err
	}

	cfg := defaultConfig()
	s := f.Section("flex")
	cfg.HubURL = s.Key("hub").MustString(cfg.HubURL)
	cfg.Password = s.Key("password").MustString(cfg.Password)
	return cfg, nil
}

func saveConfig(cfg Config) error {
	f, err := ini.Load(configPath)
	if os.IsNotExist(err) {
		f = ini.Empty()
	} else if err != nil {
		return err
	}

	s := f.Section("flex")
	s.Key("hub").SetValue(cfg.HubURL)
	s.Key("password").SetValue(cfg.Password)

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}
	return f.SaveTo(configPath)
}

var cmdConfigure = &cli.Command{
	Name:    "configure",
	Aliases: []string{"config"},
	Usage:   "Set up configuration.",
	Action: func(c *cli.Context) error {
		cfg := config

		hubURLPrompt := promptui.Prompt{
			Label:     "Flexhub URL",
			Default:   cfg.HubURL,
			AllowEdit: true,
			Validate: func(s string) error {
				parsed, err := url.Parse(s)
				if err != nil {
					return err
				}
				if parsed.Scheme != "http" && parsed.Scheme != "https" {
					return errors.New("invalid scheme")
				}
				if parsed.Host == "" {
					return errors.New("empty hostname")
				}
				return nil
			},
		}
		hubURL, err := hubURLPrompt.Run()
		if err != nil {
			return err
		}

		passwordPrompt := promptui.Prompt{
			Label:       "Password",
			Default:     cfg.Password,
			AllowEdit:   true,
			HideEntered: true,
		}
		password, err := passwordPrompt.Run()
		if err != nil {
			return err
		}

		cfg.HubURL = hubURL
		cfg.Password = password

		return saveConfig(cfg)
	},
}
