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

var (
	errNoTopic = errors.New("no MQTT topic")
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
	deviceID string
}

// NewCommandResponseHandler returns the responses handler function.
func NewCommandResponseHandler(reqCache *cache.Cache, deviceID string) message.HandlerFunc {
	h := &cmdResponseHandler{
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

			if len(deviceID) == 0 {
				topic = fmt.Sprintf("command//%s/res/%s", h.deviceID, suffix)
				msg.SetContext(connector.SetTopicToCtx(msg.Context(), topic))
			} else if cmdType == "s" {
				topic = fmt.Sprintf("command//%s/res/%s", deviceID, suffix)
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
	deviceID string,
) *message.Handler {
	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("commands_response_bus",
		TopicCommandResponse,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewCommandResponseHandler(reqCache, deviceID),
	)
}
