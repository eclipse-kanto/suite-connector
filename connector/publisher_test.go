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

package connector_test

import (
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestPublisherNotConnected(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	msg := message.NewMessage("hello", []byte("test"))

	pub := conn.NewPublisher(client, conn.QosAtMostOnce, nil, nil)
	require.NoError(t, pub.Publish("errors/test", msg))

	pub = conn.NewSyncPublisher(client, conn.QosAtLeastOnce, 10*time.Second, nil, nil)
	require.Error(t, pub.Publish("errors/test", msg))

	pub = conn.NewOnlinePublisher(client, conn.QosAtMostOnce, 10*time.Second, nil, nil)
	assert.ErrorIs(t, conn.ErrNotConnected, pub.Publish("errors/test", msg))
}
