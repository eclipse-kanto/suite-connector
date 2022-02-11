// Copyright (c) 2021 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0
//
// SPDX-License-Identifier: EPL-2.0

package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/cmd/connector/app"
	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/flags"
	"github.com/eclipse-kanto/suite-connector/logger"
)

var (
	version = "development"
)

func run(ctx context.Context, factory app.LaunchFactory, args []string) error {
	f := flag.NewFlagSet("connector", flag.ContinueOnError)

	cmd := new(config.Settings)
	flags.Add(f, cmd)
	fConfigFile := flags.AddGlobal(f)

	if err := flags.Parse(f, args, version, os.Exit); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			return flags.ErrParse
		}
		return err
	}

	settings := config.DefaultSettings()
	if err := config.ReadConfig(*fConfigFile, settings); err != nil {
		return errors.Wrap(err, "cannot parse config")
	}

	cli := flags.Copy(f)
	if err := mergo.Map(settings, cli, mergo.WithOverwriteWithEmptyValue); err != nil {
		return errors.Wrap(err, "cannot process settings")
	}

	if err := settings.ValidateStatic(); err != nil {
		return errors.Wrap(err, "settings validation error")
	}

	loggerOut, logger := logger.Setup("connector", &settings.LogSettings)
	defer loggerOut.Close()

	logger.Infof("Starting suite connector %s", version)
	flags.ConfigCheck(logger, *fConfigFile)

	if err := app.Run(ctx, factory, settings, cli, logger); err != nil {
		logger.Error("Init failure", err, nil)
		return err
	}

	return nil
}

func main() {
	if err := run(context.Background(), newLauncher, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		if errors.Is(err, flags.ErrParse) {
			os.Exit(2)
		}

		log.Fatalf("Cannot run suite connector: %v", err)
	}
}
