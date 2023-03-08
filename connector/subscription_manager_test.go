// Copyright (c) 2022 Contributors to the Eclipse Foundation
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

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"
	"go.uber.org/goleak"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicClientsTotal = "$SYS/broker/clients/total"
)

func TestAddSubscription(t *testing.T) {
	defer goleak.VerifyNone(t)

	m := connector.NewSubscriptionManager()

	config, err := testutil.NewLocalConfig()
	config.CleanSession = true
	require.NoError(t, err)

	pubClient, err := connector.NewMQTTConnection(
		config, "",
		testutil.NewLogger("connector", logger.INFO, t),
	)
	require.NoError(t, err)

	m.ForwardTo(pubClient)

	assert.True(t, m.Add(topicClientsTotal))
	assert.False(t, m.Add(topicClientsTotal))
	assert.False(t, m.Remove(topicClientsTotal))
	assert.True(t, m.Remove(topicClientsTotal))
	assert.True(t, m.Add(topicClientsTotal))
	assert.True(t, m.Remove(topicClientsTotal))
}

func TestRemoveSubscription(t *testing.T) {
	defer goleak.VerifyNone(t)

	m := connector.NewSubscriptionManager()

	assert.False(t, m.Add(topicClientsTotal))
	assert.False(t, m.Remove(topicClientsTotal))
	assert.False(t, m.Remove(topicClientsTotal))
}

func TestClearSubscriptions(t *testing.T) {
	m := connector.NewSubscriptionManager()

	config, err := testutil.NewLocalConfig()
	config.CleanSession = true
	require.NoError(t, err)

	pubClient, err := connector.NewMQTTConnection(
		config, "",
		testutil.NewLogger("connector", logger.INFO, t),
	)
	require.NoError(t, err)

	m.ForwardTo(pubClient)

	assert.True(t, m.Add(topicClientsTotal))
	m.Clear()
	assert.False(t, m.Remove(topicClientsTotal))
}
