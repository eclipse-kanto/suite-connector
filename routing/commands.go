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
	// TopicCommandRequest defines command request topic
	TopicCommandRequest = "command//+/req/#"

	// TopicCommandResponse defines command responses topic
	TopicCommandResponse = "command//+/res/#,c//+/s/#"

	msgInvalidCloudCommand = "invalid cloud command"
)

type cmdRequestHandler struct {
	reqCache *cache.Cache

	deviceID string
	tenantID string

	generic bool
}

// NewCommandRequestHandler returns the commands handler function.
func NewCommandRequestHandler(reqCache *cache.Cache,
	tenantID, deviceID string,
	generic bool,
) message.HandlerFunc {
	h := &cmdRequestHandler{
		reqCache: reqCache,
		tenantID: tenantID,
		deviceID: deviceID,
		generic:  generic,
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

		if h.generic {
			//Remove special /tenant/gatewayID/stripped(deviceID) prefix
			fixed, err := fixCommand(topic)
			if err != nil {
				return nil, err
			}
			topic = fixed

			msg.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
		}

		//Send the message to command//<deviceId>/req/<suffix>
		result := make([]*message.Message, 0)

		result = append(result, msg)

		_, prefix, suffix := util.ParseCmdTopic(topic)
		//Send the message to c//<deviceId>/q/<suffix>
		topic = fmt.Sprintf("c//%s/q/%s", prefix, suffix)
		short := message.NewMessage(msg.UUID, msg.Payload)
		short.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
		result = append(result, short)

		return result, nil
	}

	return nil, errNoTopic
}

// CommandsReqBus creates the commands request bus.
func CommandsReqBus(router *message.Router,
	mosquittoPub message.Publisher,
	honoSub message.Subscriber,
	reqCache *cache.Cache,
	tenantID, deviceID string,
	generic bool,
) *message.Handler {

	topic := TopicCommandRequest
	if generic {
		//Subscription format must be command/tenantID/gatewayID/+/req/#
		topic = fmt.Sprintf("command/%s/%s/+/req/#", tenantID, deviceID)
	}

	//Hono -> Message bus -> Mosquitto Broker -> Gateway
	return router.AddHandler("commands_request_bus",
		topic,
		honoSub,
		connector.TopicEmpty,
		mosquittoPub,
		NewCommandRequestHandler(reqCache, tenantID, deviceID, generic),
	)
}

type cmdResponseHandler struct {
	reqCache *cache.Cache

	prefix string

	deviceID string
	tenantID string

	generic bool
}

// NewCommandResponseHandler returns the responses handler function.
func NewCommandResponseHandler(
	reqCache *cache.Cache,
	prefix string,
	tenantID, deviceID string,
	generic bool,
) message.HandlerFunc {
	h := &cmdResponseHandler{
		reqCache: reqCache,
		prefix:   prefix,
		tenantID: tenantID,
		deviceID: deviceID,
		generic:  generic,
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
			_, deviceID, suffix := util.ParseCmdTopic(topic)

			var buff strings.Builder
			buff.Grow(256)

			if h.generic && len(h.prefix) > 0 {
				buff.WriteString(h.prefix)
				buff.WriteString("/")
			}

			buff.WriteString("command/")

			if h.generic {
				//Add special /tenant/gatewayID/stripped(deviceID) prefix
				buff.WriteString(h.tenantID)
				buff.WriteString("/")
				buff.WriteString(h.deviceID)
				buff.WriteString("/")
				stripped, err := stripGatewayID(h.deviceID, deviceID)
				if err != nil {
					return nil, err
				}
				buff.WriteString(stripped)
			} else {
				buff.WriteString("/")
				buff.WriteString(deviceID)
			}

			buff.WriteString("/res/")
			buff.WriteString(suffix)

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
	tenantID, deviceID string,
	generic bool,
) *message.Handler {
	prefix := os.Getenv("COMMAND_RESPONSE_TOPIC_PREFIX")

	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("commands_response_bus",
		TopicCommandResponse,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewCommandResponseHandler(reqCache, prefix, tenantID, deviceID, generic),
	)
}
