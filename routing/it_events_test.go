// Copyright (c) 2023 Contributors to the Eclipse Foundation
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

//go:build (local_integration && ignore) || !unit

package routing_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"go.uber.org/goleak"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventsBus(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("events", logger.ERROR, t)

	config, err := testutil.NewLocalConfig()
	config.CleanSession = true
	require.NoError(t, err)

	client, err := connector.NewMQTTConnection(
		config, "",
		logger,
	)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())

	defer func() {
		client.Disconnect()
		time.Sleep(time.Second)
	}()

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	require.NoError(t, err)

	source := connector.NewSubscriber(client, connector.QosAtLeastOnce, false, logger, nil)
	defer source.Close()

	sink := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(16),
		},
		logger,
	)

	routing.EventsBus(router, sink, source, "testTenant", "testDevice")
	routing.TelemetryBus(router, sink, source, "testTenant", "testDevice")

	done := make(chan bool, 1)
	go func() {
		defer func() {
			done <- true
		}()

		if err := router.Run(context.Background()); err != nil {
			logger.Error("Failed to create cloud router", err, nil)
		}
	}()

	defer func() {
		router.Close()

		<-done
	}()

	<-router.Running()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sMsg, err := sink.Subscribe(ctx, connector.TopicEmpty)
	require.NoError(t, err)

	msg := message.NewMessage("hello", []byte("{}"))
	pub := connector.NewPublisher(client, connector.QosAtLeastOnce, logger, nil)
	require.NoError(t, pub.Publish("telemetry/testTenant/testDevice", msg))

	receivedMsgs, ok := subscriber.BulkRead(sMsg, 1, time.Second*5)
	require.True(t, ok)
	for _, msg := range receivedMsgs {
		assert.Equal(t, "{}", string(msg.Payload))
	}
}
