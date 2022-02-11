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
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/logger"

	conn "github.com/eclipse-kanto/suite-connector/connector"

	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestLauncherInit(t *testing.T) {
	logger := testutil.NewLogger("testing", logger.ERROR)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	manager := conn.NewSubscriptionManager()

	l := newLauncher(client, conn.NullPublisher(), manager).(*launcher)
	defer l.Stop()

	go func() {
		l.done <- true
	}()
}
