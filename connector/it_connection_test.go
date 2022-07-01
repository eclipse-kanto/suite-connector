// Copyright (c) 2022 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0
//
// SPDX-License-Identifier: EPL-2.0

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package connector_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestCredentialsProvider(t *testing.T) {
	config, err := testutil.NewLocalConfig()

	username := config.Credentials.UserName
	pass := config.Credentials.Password

	config.Credentials.UserName = "invalid"
	config.Credentials.Password = "invalid"

	pubClient, err := conn.NewMQTTConnectionCredentialsProvider(
		config, "",
		nil,
		func() (string, string) {
			return username, pass
		},
	)
	require.NoError(t, err)

	future := pubClient.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	pubClient.Disconnect()
}
