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

package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eclipse/ditto-clients-golang/protocol"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/util"
)

const (
	// TopicCommandResponse defines command responses topic
	TopicCommandResponse = "command//+/res/#,c//+/s/#"

	msgInvalidCloudCommand = "invalid cloud command"
)

type cmdRequestHandler struct {
	reqCache *cache.Cache
	deviceID string
}

// NewCommandRequestHandler returns the commands handler function.
func NewCommandRequestHandler(reqCache *cache.Cache, deviceID string) message.HandlerFunc {
	h := &cmdRequestHandler{
		deviceID: deviceID,
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

		_, deviceID, suffix := util.ParseCmdTopic(topic)

		result := make([]*message.Message, 0)

		//Send the message to command//<deviceId>/req/<suffix>
		result = append(result, msg)

		//Send the message to c//<deviceId>/q/<suffix>
		topic = fmt.Sprintf("c//%s/q/%s", deviceID, suffix)
		short := message.NewMessage(msg.UUID, msg.Payload)
		short.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
		result = append(result, short)

		if deviceID == h.deviceID {
			//Send the message to command///req/<suffix>
			topic = fmt.Sprintf("command///req/%s", suffix)
			longNoID := message.NewMessage(msg.UUID, msg.Payload)
			longNoID.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
			result = append(result, longNoID)

			//Send the message to c///q/<suffix>
			topic = fmt.Sprintf("c///q/%s", suffix)
			shortNoID := message.NewMessage(msg.UUID, msg.Payload)
			shortNoID.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
			result = append(result, shortNoID)
		}

		return result, nil
	}

	return nil, errNoTopic
}

// CommandsReqBus creates the commands request bus.
func CommandsReqBus(router *message.Router,
	mosquittoPub message.Publisher,
	honoSub message.Subscriber,
	reqCache *cache.Cache,
	deviceID string,
) *message.Handler {
	//Hono -> Message bus -> Mosquitto Broker -> Gateway
	return router.AddHandler("commands_request_bus",
		connector.TopicEmpty,
		honoSub,
		connector.TopicEmpty,
		mosquittoPub,
		NewCommandRequestHandler(reqCache, deviceID),
	)
}

type cmdResponseHandler struct {
	reqCache *cache.Cache

	prefix   string
	deviceID string
}

// NewCommandResponseHandler returns the responses handler function.
func NewCommandResponseHandler(reqCache *cache.Cache, prefix string, deviceID string) message.HandlerFunc {
	h := &cmdResponseHandler{
		prefix:   prefix,
		deviceID: deviceID,
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
			cmdType, deviceID, suffix := util.ParseCmdTopic(topic)

			var buff strings.Builder
			buff.Grow(64)

			if len(h.prefix) > 0 {
				buff.WriteString(h.prefix)
				buff.WriteString("/")
			}

			if len(deviceID) == 0 {
				buff.WriteString("command//")
				buff.WriteString(h.deviceID)
				buff.WriteString("/res/")
				buff.WriteString(suffix)
			} else if cmdType == "s" {
				buff.WriteString("command//")
				buff.WriteString(deviceID)
				buff.WriteString("/res/")
				buff.WriteString(suffix)
			} else {
				buff.WriteString(topic)
			}

			msg.SetContext(connector.SetTopicToCtx(msg.Context(), buff.String()))

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
	deviceID string,
) *message.Handler {
	prefix := os.Getenv("COMMAND_RESPONSE_TOPIC_PREFIX")

	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("commands_response_bus",
		TopicCommandResponse,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewCommandResponseHandler(reqCache, prefix, deviceID),
	)
}
