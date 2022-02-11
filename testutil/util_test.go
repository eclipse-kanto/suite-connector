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

package testutil_test

import (
	"os"
	"testing"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	l := testutil.NewLogger("testutil", logger.TRACE)
	assert.NotNil(t, l)
}

func TestNewLoggerEnvNop(t *testing.T) {
	nop := os.Getenv("TEST_NOP_LOGGER")
	defer os.Setenv("TEST_NOP_LOGGER", nop)

	err := os.Setenv("TEST_NOP_LOGGER", "val")
	require.NoError(t, err)

	l := testutil.NewLogger("testutil", logger.DEBUG)
	assert.NotNil(t, l)
}

func TestNewLocalConfig(t *testing.T) {
	conf, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	assert.NotNil(t, conf)
}

func TestNewLocalConfigInvalidURI(t *testing.T) {
	uri := os.Getenv("TEST_MQTT_URI")
	defer os.Setenv("TEST_MQTT_URI", uri)

	err := os.Setenv("TEST_MQTT_URI", "invalid")
	require.NoError(t, err)

	conf, err := testutil.NewLocalConfig()
	assert.Error(t, err)
	assert.Nil(t, conf)
}
