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
	"testing"

	"github.com/eclipse-kanto/suite-connector/cmd/connector/app"
	"github.com/eclipse-kanto/suite-connector/flags"

	"github.com/stretchr/testify/require"
)

func TestRunWithInvalidArgument(t *testing.T) {
	args := []string{"-invalid=invalid"}

	require.ErrorIs(t, run(context.Background(), app.NewMockLauncher, args), flags.ErrParse)
}

func TestRunWithInvalidConfigFile(t *testing.T) {
	args := []string{"-configFile=main_test.go"}

	require.Error(t, run(context.Background(), app.NewMockLauncher, args))
}

func TestRunWithNoCA(t *testing.T) {
	args := []string{"-cacert=invalid.crt"}

	require.Error(t, run(context.Background(), app.NewMockLauncher, args))
}

func TestRunHelp(t *testing.T) {
	args := []string{"-h"}

	require.ErrorIs(t, run(context.Background(), app.NewMockLauncher, args), flag.ErrHelp)
}
