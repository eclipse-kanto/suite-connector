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

package routing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/eclipse-kanto/suite-connector/routing"
)

func TestNotifyListeners(t *testing.T) {
	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	params := routing.NewGwParams("deviceId", "tenantId", "policyId")

	sink := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(16)},
		logger,
	)
	defer sink.Close()

	client, err := connector.NewMQTTConnection(config, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	statusHandler := &routing.ConnectionStatusHandler{
		Pub:    sink,
		Logger: logger,
	}
	client.AddConnectionListener(statusHandler)

	reconnectHandler := &routing.ReconnectHandler{
		Pub:    sink,
		Params: params,
		Logger: logger,
	}
	client.AddConnectionListener(reconnectHandler)

	sMsg, err := sink.Subscribe(context.Background(), routing.TopicConnectionStatus)
	require.NoError(t, err)

	pMsg, err := sink.Subscribe(context.Background(), routing.TopicGwParamsResponse)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()
	defer client.RemoveConnectionListener(statusHandler)
	defer client.RemoveConnectionListener(reconnectHandler)

	if receivedMsgs, ok := subscriber.BulkRead(sMsg, 1, time.Second*5); ok {
		status := new(routing.ConnectionStatus)
		err := json.Unmarshal(receivedMsgs[0].Payload, status)
		require.NoError(t, err)

		assert.True(t, status.Connected)
		assert.Equal(t, 0, len(status.Cause))
	} else {
		require.Fail(t, "Status message not delivered")
	}

	if receivedMsgs, ok := subscriber.BulkRead(pMsg, 1, time.Second*5); ok {
		expected, err := json.Marshal(params)
		require.NoError(t, err)

		assert.JSONEq(t, string(expected), string(receivedMsgs[0].Payload))
	} else {
		require.Fail(t, "Params message not delivered")
	}
}

func TestParamsBus(t *testing.T) {
	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	source := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(16)},
		logger,
	)

	sink := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(16)},
		logger,
	)

	r, err := message.NewRouter(
		message.RouterConfig{
			CloseTimeout: 30 * time.Second,
		},
		logger,
	)
	require.NoError(t, err)

	params := routing.NewGwParams("deviceId", "tenantId", "policyId")
	routing.ParamsBus(r, params, sink, source, logger)

	go func() {
		if err := r.Run(context.Background()); err != nil {
			logger.Error("Failed to start router", err, nil)
		}
	}()
	defer r.Close()

	<-r.Running()

	msgs, err := sink.Subscribe(context.Background(), routing.TopicGwParamsResponse)
	require.NoError(t, err)

	msg := message.NewMessage(watermill.NewUUID(), []byte("{}"))
	require.NoError(t, source.Publish(routing.TopicGwParamsRequest, msg))

	if receivedMsgs, ok := subscriber.BulkRead(msgs, 1, time.Second*5); ok {
		expected, err := json.Marshal(params)
		require.NoError(t, err)

		assert.JSONEq(t, string(expected), string(receivedMsgs[0].Payload))
	} else {
		require.Fail(t, "Message not delivered")
	}
}

func TestServiceRouter(t *testing.T) {
	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	config.CleanSession = true

	client, err := connector.NewMQTTConnection(config, "", logger)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	manager := connector.NewSubscriptionManager()
	r := routing.CreateServiceRouter(client, manager, logger)
	defer r.Close()
}
