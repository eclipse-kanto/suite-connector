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

package routing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/eclipse-kanto/suite-connector/routing"
)

func TestStatusCause(t *testing.T) {
	type causeTest struct {
		mqttErr error
		cause   string
		retry   bool
	}
	tests := []causeTest{
		{
			mqttErr: packets.ErrorRefusedBadProtocolVersion,
			cause:   routing.StatusConnectionError,
		},
		{
			mqttErr: packets.ErrorRefusedIDRejected,
			cause:   routing.StatusConnectionError,
		},
		{
			mqttErr: packets.ErrorRefusedBadUsernameOrPassword,
			cause:   routing.StatusConnectionNotAuthenticated,
		},
		{
			mqttErr: packets.ErrorRefusedNotAuthorised,
			cause:   routing.StatusConnectionError,
			retry:   false,
		},
		{
			mqttErr: packets.ErrorNetworkError,
			cause:   routing.StatusConnectionError,
			retry:   true,
		},
		{
			mqttErr: errors.New("unknown"), // unknown
			cause:   routing.StatusConnectionError,
			retry:   true,
		},
	}
	for _, test := range tests {
		cause, retry := routing.StatusCause(test.mqttErr)
		assert.EqualValues(t, test.cause, cause)
		assert.EqualValues(t, test.retry, retry)
	}
}

func TestErrorsHandler(t *testing.T) {
	logger := testutil.NewLogger("routing", logger.ERROR)

	sink := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(2),
		},
		logger,
	)
	defer sink.Close()

	msgs, err := sink.Subscribe(context.Background(), routing.TopicConnectionStatus)
	require.NoError(t, err)

	h := &routing.ErrorsHandler{
		StatusPub: sink,
		Logger:    logger,
	}

	h.Connected(false, packets.ErrorRefusedBadUsernameOrPassword)
	checkStatus(t, msgs, routing.StatusConnectionNotAuthenticated)

	h.Connected(false, packets.ErrorNetworkError)
	checkStatus(t, msgs, routing.StatusConnectionError)
}

func checkStatus(t *testing.T, msgs <-chan *message.Message, cause string) {
	if receivedMsgs, ok := subscriber.BulkRead(msgs, 1, time.Second*5); ok {
		status := new(routing.ConnectionStatus)
		require.NoError(t, json.Unmarshal(receivedMsgs[0].Payload, status))

		assert.False(t, status.Connected)
		assert.Equal(t, cause, status.Cause)
	} else {
		require.Fail(t, "Status message not delivered")
	}
}

func TestPublishError(t *testing.T) {
	logger := testutil.NewLogger("routing", logger.ERROR)

	sink := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(1),
		},
		logger,
	)
	require.NoError(t, sink.Close())

	routing.SendStatus(routing.StatusConfigError, sink, logger)

	params := routing.NewGwParams("A", "B", "C")
	routing.SendGwParams(params, true, sink, logger)
}
