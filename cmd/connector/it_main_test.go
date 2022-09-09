// Copyright (c) 2021 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0, or the Apache License, Version 2.0
// which is available at https://www.apache.org/licenses/LICENSE-2.0.
//
// SPDX-License-Identifier: EPL-2.0 OR Apache-2.0

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/cmd/connector/app"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestRunWithMockFactory(t *testing.T) {
	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	localAddr := fmt.Sprintf("-localAddress=%s", config.URL)
	localUsername := fmt.Sprintf("-localUsername=%s", config.Credentials.UserName)
	localPassword := fmt.Sprintf("-localPassword=%s", config.Credentials.Password)

	args := []string{"-caCert=main_test.go", "-logFile=", localAddr, localUsername, localPassword}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, run(ctx, app.NewMockLauncher, args))
}

func TestRunErrorWithMockFactory(t *testing.T) {
	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	localAddr := fmt.Sprintf("-localAddress=%s", config.URL)

	args := []string{"-caCert=main_test.go", "-logFile=", localAddr}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.Error(t, run(ctx, app.NewMockLauncher, args))
}
