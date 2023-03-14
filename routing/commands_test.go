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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/eclipse/ditto-clients-golang/model"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/eclipse/ditto-clients-golang/protocol/things"

	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/util"
)

const (
	namespace     = "routing.test"
	name          = "device"
	correlationId = "test/routing/commands"
	msgUid        = "test_msg"
)

var (
	id       = model.NewNamespacedID(namespace, name)
	deviceId = id.String()
	response = things.NewCommand(id)
)

type CommandHandlersTestSuite struct {
	suite.Suite
	reqCache        *cache.Cache
	requestHandler  message.HandlerFunc
	responseHandler message.HandlerFunc
}

func TestCommandHandlersTestSuite(t *testing.T) {
	suite.Run(t, new(CommandHandlersTestSuite))
}

func (s *CommandHandlersTestSuite) SetupSuite() {
	s.reqCache = cache.NewTTLCache()
	s.requestHandler = routing.NewCommandRequestHandler(s.reqCache, deviceId)
	s.responseHandler = routing.NewCommandResponseHandler(s.reqCache, deviceId)

	response.Delete()
}

func (s *CommandHandlersTestSuite) TearDownSuite() {
	s.reqCache.Close()
}

type requestTest struct {
	initialTopic   string
	shortTopic     string
	longTopic      string
	longTopicNoId  string
	shortTopicNoId string
}

func (s *CommandHandlersTestSuite) TestHandleRequest() {
	cmd := things.NewCommand(id)
	cmd.Delete()
	env := cmd.Envelope(protocol.WithCorrelationID(correlationId))
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)
	msg := message.NewMessage(msgUid, payload)

	tests := []requestTest{
		{
			initialTopic:   fmt.Sprintf("command//%s/req/#", deviceId),
			longTopic:      fmt.Sprintf("command//%s/req/#", deviceId),
			shortTopic:     fmt.Sprintf("c//%s/q/#", deviceId),
			longTopicNoId:  "command///req/#",
			shortTopicNoId: "c///q/#",
		},
	}

	for _, test := range tests {
		msg.SetContext(connector.SetTopicToCtx(context.Background(), test.initialTopic))
		messages, err := s.requestHandler(msg)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), 4, len(messages), test.initialTopic)

		longTopic, _ := connector.TopicFromCtx(messages[0].Context())
		assert.Equal(s.T(), test.longTopic, longTopic)

		shortTopic, _ := connector.TopicFromCtx(messages[1].Context())
		assert.Equal(s.T(), test.shortTopic, shortTopic)
		assert.Equal(s.T(), messages[0].Payload, messages[1].Payload)

		longTopicNoId, _ := connector.TopicFromCtx(messages[2].Context())
		assert.Equal(s.T(), test.longTopicNoId, longTopicNoId)
		assert.Equal(s.T(), messages[0].Payload, messages[2].Payload)

		shortTopicNoId, _ := connector.TopicFromCtx(messages[3].Context())
		assert.Equal(s.T(), test.shortTopicNoId, shortTopicNoId)
		assert.Equal(s.T(), messages[0].Payload, messages[3].Payload)

		if len(env.Headers.CorrelationID()) > 0 {
			assertCache(s, env.Headers.CorrelationID(), true)
		}
	}
}

func (s *CommandHandlersTestSuite) TestHandleRequestInvalidPayload() {
	msgInvalidPayload := message.NewMessage(msgUid, []byte("invalid"))
	msgInvalidPayload.SetContext(connector.SetTopicToCtx(context.Background(), "t"))
	assertMessageInvalid(s, s.requestHandler, msgInvalidPayload)
}

func (s *CommandHandlersTestSuite) TestHandleRequestNoTopic() {
	assertMessageInvalid(s, s.requestHandler, message.NewMessage(msgUid, []byte{}))
}

type responseTest struct {
	initialTopic      string
	topicAfterHandler string
}

func (s *CommandHandlersTestSuite) TestHandleResponseWithCorrelationId() {
	env := response.Envelope(protocol.WithCorrelationID(correlationId)).
		WithValue(1).
		WithStatus(http.StatusOK)
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)
	msg := message.NewMessage(msgUid, payload)

	const resTopicDevice = "command///res/test_request/200"
	resTopicVirtualDevice := fmt.Sprintf("command//%s/res/test_request/200", deviceId)
	resTests := []responseTest{
		{
			initialTopic:      resTopicDevice,
			topicAfterHandler: resTopicVirtualDevice,
		},
		{
			initialTopic: resTopicVirtualDevice,
		},
		{
			initialTopic:      fmt.Sprintf("c//%s/s/test_request/200", deviceId),
			topicAfterHandler: resTopicVirtualDevice,
		},
		{
			initialTopic:      "c///s/test_request/200",
			topicAfterHandler: resTopicVirtualDevice,
		},
	}

	for _, test := range resTests {
		msg.SetContext(connector.SetTopicToCtx(context.Background(), test.initialTopic))
		s.reqCache.Put(correlationId, true, util.ParseTimeout(env.Headers.Timeout()))

		messages, err := s.responseHandler(msg)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), len(messages), 1)

		actualTopic, _ := connector.TopicFromCtx(messages[0].Context())
		if len(test.topicAfterHandler) > 0 {
			assert.Equal(s.T(), test.topicAfterHandler, actualTopic)
		} else {
			assert.Equal(s.T(), test.initialTopic, actualTopic)
		}
		assert.Equal(s.T(), msg.Payload, messages[0].Payload)
		assertCache(s, env.Headers.CorrelationID(), false)
	}
}

func (s *CommandHandlersTestSuite) TestHandleResponseNoCorrelationId() {
	env := response.Envelope(protocol.WithChannel("test"))
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)

	msg := message.NewMessage(msgUid, payload)
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "topic"))
	messages, err := s.responseHandler(msg)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), messages)
}

func (s *CommandHandlersTestSuite) TestHandleResponseNoHeaders() {
	env := response.Envelope()
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)

	msg := message.NewMessage(msgUid, payload)
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "topic"))
	messages, err := s.responseHandler(msg)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), messages)
}

func (s *CommandHandlersTestSuite) TestHandleResponseInvalidPayload() {
	msgInvalidPayload := message.NewMessage(msgUid, []byte("invalid"))
	msgInvalidPayload.SetContext(connector.SetTopicToCtx(context.Background(), "t"))
	assertMessageInvalid(s, s.responseHandler, msgInvalidPayload)
}

func (s *CommandHandlersTestSuite) TestHandleResponseNoTopic() {
	assertMessageInvalid(s, s.responseHandler, message.NewMessage(msgUid, []byte{}))
}

func (s *CommandHandlersTestSuite) TestHandlersCache() {
	const commonCorrelationId = "test/routing/requestAndResponse"
	const otherCorrelationId = "test/routing/response"
	const requestTopic = "command///req/#"
	const responseTopic = "command///res/test_request/200"

	cmd := things.NewCommand(id)
	cmd.Delete()
	env := cmd.Envelope(protocol.WithCorrelationID(commonCorrelationId))
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)
	msg := message.NewMessage(msgUid, payload)
	msg.SetContext(connector.SetTopicToCtx(context.Background(), requestTopic))

	_, err = s.requestHandler(msg)
	require.NoError(s.T(), err)
	assertCache(s, commonCorrelationId, true)

	env = cmd.Envelope(protocol.WithCorrelationID(otherCorrelationId))
	payload, err = json.Marshal(env)
	require.NoError(s.T(), err)
	msg = message.NewMessage(msgUid, payload)
	msg.SetContext(connector.SetTopicToCtx(context.Background(), responseTopic))
	s.reqCache.Put(otherCorrelationId, true, util.ParseTimeout(env.Headers.Timeout()))

	_, err = s.responseHandler(msg)
	require.NoError(s.T(), err)
	assertCache(s, commonCorrelationId, true)

	env = cmd.Envelope(protocol.WithCorrelationID(commonCorrelationId))
	payload, err = json.Marshal(env)
	require.NoError(s.T(), err)
	msg = message.NewMessage(msgUid, payload)
	msg.SetContext(connector.SetTopicToCtx(context.Background(), responseTopic))
	s.reqCache.Put(commonCorrelationId, true, util.ParseTimeout(env.Headers.Timeout()))

	_, err = s.responseHandler(msg)
	require.NoError(s.T(), err)
	assertCache(s, commonCorrelationId, false)
}

func assertMessageInvalid(s *CommandHandlersTestSuite, handler message.HandlerFunc, message *message.Message) {
	messages, err := handler(message)
	assert.Nil(s.T(), messages)
	assert.Error(s.T(), err)
}

func assertCache(s *CommandHandlersTestSuite, correlationId string, present bool) {
	_, correlationIdPresent := s.reqCache.Get(correlationId)
	assert.Equal(s.T(), present, correlationIdPresent, correlationId)
}
