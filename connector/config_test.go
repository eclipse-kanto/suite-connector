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

package connector_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestInvalidURL(t *testing.T) {
	_, err := conn.NewMQTTClientConfig("mqtts://localhost\\test")
	assert.Error(t, err)

	cfg := new(conn.Configuration)
	assert.Error(t, cfg.Validate())

	cfg = &conn.Configuration{
		URL: "mqtts://localhost\\test",
	}
	assert.Error(t, cfg.Validate())
}

func TestReconnectIntervalValid(t *testing.T) {
	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	config.AuthErrRetries = 3
	config.BackoffMultiplier = 1.5
	config.MinReconnectInterval = 2 * time.Second
	config.MaxReconnectInterval = 10 * time.Second
	assert.NoError(t, config.Validate())

	config.MinReconnectInterval = 0
	config.MaxReconnectInterval = 0
	assert.NoError(t, config.Validate())
}

func TestReconnectIntervalInvalid(t *testing.T) {
	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	config.MinReconnectInterval = time.Duration(-1)
	assert.Error(t, config.Validate())

	config.MinReconnectInterval = 10 * time.Second
	config.MaxReconnectInterval = time.Duration(-1)
	assert.Error(t, config.Validate())

	config.MaxReconnectInterval = 2 * time.Second
	assert.Error(t, config.Validate())

	config.MaxReconnectInterval = 10 * time.Second
	config.BackoffMultiplier = 0.2
	assert.Error(t, config.Validate())

	config.BackoffMultiplier = 2.0
	config.AuthErrRetries = 0
	assert.Error(t, config.Validate())
}
