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

package routing_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"go.uber.org/goleak"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sinkTopic = "sink/"
)

func TestEventsBus(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("events", logger.ERROR, t)

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	require.NoError(t, err)

	source := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(16),
		},
		logger,
	)

	sink := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(16),
		},
		logger,
	)

	pub := newSinkDecorator(sinkTopic, sink)
	routing.EventsBus(router, pub, source)
	routing.TelemetryBus(router, pub, source)

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
	sMsg, err := sink.Subscribe(ctx, sinkTopic)
	require.NoError(t, err)

	msg := message.NewMessage("hello", []byte("{}"))
	require.NoError(t, source.Publish(routing.TopicsEvent, msg))
	require.NoError(t, source.Publish(routing.TopicsTelemetry, msg))

	receivedMsgs, ok := subscriber.BulkRead(sMsg, 2, time.Second*3)
	require.True(t, ok)
	for _, msg := range receivedMsgs {
		assert.Equal(t, "{}", string(msg.Payload))
	}
}

func newSinkDecorator(sinkTopic string, pub message.Publisher) message.Publisher {
	return &sinkdecorator{
		sinkTopic: sinkTopic,
		pub:       pub,
	}
}

type sinkdecorator struct {
	sinkTopic string
	pub       message.Publisher
}

func (p *sinkdecorator) Publish(topic string, messages ...*message.Message) error {
	return p.pub.Publish(p.sinkTopic, messages...)
}

func (p *sinkdecorator) Close() error {
	return p.pub.Close()
}
