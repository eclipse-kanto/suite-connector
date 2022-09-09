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
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/eclipse/ditto-clients-golang/model"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/eclipse/ditto-clients-golang/protocol/things"

	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/testutil"
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
	s.requestHandler = routing.NewCommandRequestHandler(s.reqCache)
	s.responseHandler = routing.NewCommandResponseHandler(s.reqCache)

	response.Delete()
}

func (s *CommandHandlersTestSuite) TearDownSuite() {
	s.reqCache.Close()
}

type handlerTest struct {
	initialTopic      string
	topicAfterHandler string
}

func (s *CommandHandlersTestSuite) TestHandleRequest() {
	cmd := things.NewCommand(id)
	cmd.Delete()
	env := cmd.Envelope(protocol.WithCorrelationID(correlationId))
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)
	msg := message.NewMessage(msgUid, payload)

	tests := []handlerTest{
		{
			initialTopic:      fmt.Sprintf("command//%s/req/#", deviceId),
			topicAfterHandler: fmt.Sprintf("c//%s/q/#", deviceId),
		},
		{
			initialTopic:      "command///req/#",
			topicAfterHandler: "c///q/#",
		},
	}

	for _, test := range tests {
		msg.SetContext(connector.SetTopicToCtx(context.Background(), test.initialTopic))
		messages, err := s.requestHandler(msg)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), 2, len(messages), test.initialTopic)
		assert.Equal(s.T(), msg, messages[0])
		broadcastTopic, _ := connector.TopicFromCtx(messages[1].Context())
		assert.Equal(s.T(), test.topicAfterHandler, broadcastTopic)
		assert.Equal(s.T(), messages[0].Payload, messages[1].Payload)

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

func (s *CommandHandlersTestSuite) TestHandleResponseWithCorrelationId() {
	env := response.Envelope(protocol.WithCorrelationID(correlationId)).
		WithValue(1).
		WithStatus(http.StatusOK)
	payload, err := json.Marshal(env)
	require.NoError(s.T(), err)
	msg := message.NewMessage(msgUid, payload)

	const resTopicDevice = "command///res/test_request/200"
	resTopicVirtualDevice := fmt.Sprintf("command//%s/res/test_request/200", deviceId)
	resTests := []handlerTest{
		{
			initialTopic: resTopicDevice,
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
			topicAfterHandler: resTopicDevice,
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

func TestCommandsBus(t *testing.T) {
	logger := testutil.NewLogger("commands", logger.ERROR, t)

	source := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(16)},
		logger,
	)

	sink := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(16)},
		logger,
	)

	r, err := message.NewRouter(message.RouterConfig{}, logger)
	require.NoError(t, err)

	c := cache.NewTTLCache()
	defer c.Close()

	routing.CommandsReqBus(r, newSinkDecorator(routing.TopicCommandRequest, sink), source, c)
	routing.CommandsResBus(r, newSinkDecorator(routing.TopicCommandResponse, sink), source, c)

	r.AddMiddleware(func(h message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			msg.SetContext(connector.SetTopicToCtx(msg.Context(), routing.TopicCommandRequest))
			return h(msg)
		}
	})

	done := make(chan bool, 1)
	go func() {
		defer func() {
			done <- true
		}()

		if err := r.Run(context.Background()); err != nil {
			logger.Error("Failed to create commands router", err, nil)
		}
	}()

	defer func() {
		r.Close()

		<-done
	}()

	<-r.Running()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pMsg, err := sink.Subscribe(ctx, routing.TopicCommandRequest)
	require.NoError(t, err)

	cmd := things.NewCommand(id)
	cmd.Delete()

	env := cmd.Envelope(protocol.WithCorrelationID(watermill.NewUUID()))
	payload, err := json.Marshal(env)
	require.NoError(t, err)

	msg := message.NewMessage(env.Headers.CorrelationID(), payload)
	require.NoError(t, source.Publish(routing.TopicCommandRequest, msg))

	if _, ok := subscriber.BulkRead(pMsg, 1, time.Second*5); !ok {
		require.Fail(t, "Command request not delivered")
	}
}
