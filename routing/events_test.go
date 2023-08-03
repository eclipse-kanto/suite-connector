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

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	sinkTopic = "sink/"
)

type EventHandlersTestSuite struct {
	suite.Suite

	eventsHandler message.HandlerFunc
}

func (s *EventHandlersTestSuite) TestHandleEventNoTopic() {
	messages, err := s.eventsHandler(message.NewMessage(watermill.NewShortUUID(), []byte("{}")))
	assert.Nil(s.T(), messages)
	assert.Error(s.T(), err)
}

type eventsTest struct {
	initialTopic      string
	topicAfterHandler string
}

func (s *EventHandlersTestSuite) TestHandleEvents() {
	eventsTest := []eventsTest{
		{
			initialTopic:      "e",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "t",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "e/",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "t/",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "event",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "telemetry",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "event/",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "telemetry/",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "e/testTenant/testDevice",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "t/testTenant/testDevice",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "e//testDevice",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "t//testDevice",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "event//testDevice",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "telemetry//testDevice",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "event/testTenant/testDevice",
			topicAfterHandler: "event/testTenant/testDevice",
		},
		{
			initialTopic:      "telemetry/testTenant/testDevice",
			topicAfterHandler: "telemetry/testTenant/testDevice",
		},
		{
			initialTopic:      "event/testTenant/testAnotherDevice",
			topicAfterHandler: "event/testTenant/testAnotherDevice",
		},
		{
			initialTopic:      "telemetry/testTenant/testAnotherDevice",
			topicAfterHandler: "telemetry/testTenant/testAnotherDevice",
		},
		{
			initialTopic:      "event/testTenant/testAnotherDevice/testing",
			topicAfterHandler: "event/testTenant/testAnotherDevice/testing",
		},
		{
			initialTopic:      "telemetry/testTenant/testAnotherDevice/testing",
			topicAfterHandler: "telemetry/testTenant/testAnotherDevice/testing",
		},
	}

	for _, test := range eventsTest {
		msg := message.NewMessage(watermill.NewShortUUID(), []byte("{}"))
		msg.SetContext(connector.SetTopicToCtx(context.Background(), test.initialTopic))

		messages, err := s.eventsHandler(msg)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), len(messages), 1)

		actualTopic, ok := connector.TopicFromCtx(messages[0].Context())
		assert.True(s.T(), ok)
		assert.Equal(s.T(), test.topicAfterHandler, actualTopic)
		assert.Equal(s.T(), msg.Payload, messages[0].Payload)
	}
}

func TestEventHandlersTestSuite(t *testing.T) {
	s := &EventHandlersTestSuite{
		eventsHandler: routing.NewEventsHandler("", "testTenant", "testDevice", false),
	}
	suite.Run(t, s)
}

func TestEventTopicGeneric(t *testing.T) {
	msg := message.NewMessage(watermill.NewShortUUID(), []byte("{}"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "event/testTenant/testGateway:testDevice/testing"))

	eventsHandler := routing.NewEventsHandler("prefix", "testTenant", "testGateway", true)
	messages, err := eventsHandler(msg)
	require.NoError(t, err)
	assert.Equal(t, len(messages), 1)

	actualTopic, ok := connector.TopicFromCtx(messages[0].Context())
	assert.True(t, ok)
	assert.Equal(t, "prefix/event/testTenant/testGateway/testDevice/testing", actualTopic)
}
