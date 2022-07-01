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

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package connector_test

import (
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/cenkalti/backoff/v3"
	"go.uber.org/goleak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestWithInvalidConfig(t *testing.T) {
	defer goleak.VerifyNone(t)

	_, err := conn.NewMQTTConnection(nil, "", nil)
	assert.Error(t, err)

	cfg := &conn.Configuration{
		URL: "mqtts://localhost\\test",
	}
	_, err = conn.NewMQTTConnection(cfg, "", nil)
	assert.Error(t, err)
}

func TestConnectBackoff(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	config.MinReconnectInterval = time.Minute
	config.MaxReconnectInterval = 4 * time.Minute
	config.BackoffMultiplier = 2

	config.WillTopic = "test/backoff"
	config.WillQos = conn.QosAtMostOnce
	config.WillMessage = []byte("testing")

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	exp := client.ConnectBackoff()

	var expectedResults = []time.Duration{60, 120, 240, 240, 240, 240, 240, 240, 240, 240}
	for i, d := range expectedResults {
		expectedResults[i] = d * time.Second
	}
	verifyBackoff(t, exp, expectedResults)
}

func TestLinearBackoff(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	config.MinReconnectInterval = 2 * time.Minute
	config.MaxReconnectInterval = 2 * time.Minute
	config.BackoffMultiplier = 1

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	exp := client.ConnectBackoff()

	var expectedResults = []time.Duration{120, 120, 120, 120, 120, 120, 120, 120, 120, 120}
	for i, d := range expectedResults {
		expectedResults[i] = d * time.Second
	}
	verifyBackoff(t, exp, expectedResults)
}

func verifyBackoff(t *testing.T, b backoff.BackOff, expectedResults []time.Duration) {
	b.Reset()
	for _, expected := range expectedResults {
		var minInterval = expected - time.Duration(0.25*float64(expected))
		var maxInterval = expected + time.Duration(0.25*float64(expected))

		var actualInterval = b.NextBackOff()
		assert.NotEqual(t, backoff.Stop, actualInterval)
		assert.LessOrEqual(t, minInterval, actualInterval)
		assert.GreaterOrEqual(t, maxInterval, actualInterval)
	}
}

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
