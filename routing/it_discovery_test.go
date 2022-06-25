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

//go:build integration_hub
// +build integration_hub

package routing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamsDiscovery(t *testing.T) {
	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	require.NoError(t, client.ConnectSync(context.Background()))
	defer client.Disconnect()

	pub := conn.NewPublisher(client, conn.QosAtMostOnce, nil, nil)
	defer pub.Close()

	sub := conn.NewBufferedSubscriber(client, 16, false, nil, nil)
	defer sub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages, err := sub.Subscribe(ctx, routing.TopicGwParamsResponse)
	require.NoError(t, err)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	params := make(map[string]interface{}, 3)

loop:
	for {
		select {
		case <-ticker.C:
			pub.Publish(routing.TopicGwParamsRequest, message.NewMessage("hello", []byte("{}")))

		case msg := <-messages:
			if err := json.Unmarshal(msg.Payload, &params); err != nil {
				panic(err)
			}
			break loop

		case <-ctx.Done():
			break loop
		}
	}

	assert.Contains(t, params, "policyId")
	assert.Contains(t, params, "deviceId")
	assert.Contains(t, params, "tenantId")
}
