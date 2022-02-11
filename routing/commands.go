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

package routing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eclipse/ditto-clients-golang/protocol"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/util"
)

const (
	// TopicCommandRequest defines command request topic
	TopicCommandRequest = "command//+/req/#"

	// TopicCommandResponse defines command responses topic
	TopicCommandResponse = "command//+/res/#,c//+/s/#"

	msgInvalidCloudCommand = "invalid cloud command"
)

var (
	errNoTopic = errors.New("no MQTT topic")
)

type cmdRequestHandler struct {
	reqCache *cache.Cache
}

// NewCommandRequestHandler returns the commands handler function.
func NewCommandRequestHandler(reqCache *cache.Cache) message.HandlerFunc {
	h := &cmdRequestHandler{
		reqCache: reqCache,
	}

	return h.HandleRequest
}

// HandleRequest handles a published command.
func (h *cmdRequestHandler) HandleRequest(msg *message.Message) ([]*message.Message, error) {
	if topic, ok := connector.TopicFromCtx(msg.Context()); ok {
		command := protocol.Envelope{Headers: protocol.NewHeaders()}
		if err := json.Unmarshal(msg.Payload, &command); err != nil {
			return nil, errors.Wrap(err, msgInvalidCloudCommand)
		}

		correlationID := command.Headers.CorrelationID()
		if len(correlationID) > 0 {
			timeout := util.ParseTimeout(command.Headers.Timeout())
			if timeout != 0 {
				h.reqCache.Put(correlationID, true, timeout+time.Second*5)
			}
		}

		_, prefix, suffix := util.ParseCmdTopic(topic)
		topic = fmt.Sprintf("c//%s/q/%s", prefix, suffix)
		broadcast := message.NewMessage(msg.UUID, msg.Payload)
		broadcast.SetContext(connector.SetTopicToCtx(msg.Context(), topic))

		return []*message.Message{msg, broadcast}, nil
	}

	return nil, errNoTopic
}

// CommandsReqBus creates the commands request bus.
func CommandsReqBus(router *message.Router,
	mosquittoPub message.Publisher,
	honoSub message.Subscriber,
	reqCache *cache.Cache,
) *message.Handler {
	//Hono -> Message bus -> Mosquitto Broker -> Gateway
	return router.AddHandler("commands_request_bus",
		TopicCommandRequest,
		honoSub,
		connector.TopicEmpty,
		mosquittoPub,
		NewCommandRequestHandler(reqCache),
	)
}

type cmdResponseHandler struct {
	reqCache *cache.Cache
}

// NewCommandResponseHandler returns the responses handler function.
func NewCommandResponseHandler(reqCache *cache.Cache) message.HandlerFunc {
	h := &cmdResponseHandler{
		reqCache: reqCache,
	}

	return h.HandleResponse
}

// HandleResponse manages the commands response.
func (h *cmdResponseHandler) HandleResponse(msg *message.Message) ([]*message.Message, error) {
	if topic, ok := connector.TopicFromCtx(msg.Context()); ok {
		command := protocol.Envelope{Headers: protocol.NewHeaders()}
		if err := json.Unmarshal(msg.Payload, &command); err != nil {
			return nil, errors.Wrap(err, msgInvalidCloudCommand)
		}

		correlationID := command.Headers.CorrelationID()
		if h.reqCache.Remove(correlationID) {
			if cmdType, prefix, suffix := util.ParseCmdTopic(topic); cmdType == "s" {
				topic = fmt.Sprintf("command//%s/res/%s", prefix, suffix)
				msg.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
			}

			return []*message.Message{msg}, nil
		}

		return nil, nil
	}

	return nil, errNoTopic
}

// CommandsResBus creates the commands response bus.
func CommandsResBus(router *message.Router,
	honoPub message.Publisher,
	mosquittoSub message.Subscriber,
	reqCache *cache.Cache,
) *message.Handler {
	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("commands_response_bus",
		TopicCommandResponse,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewCommandResponseHandler(reqCache),
	)
}
