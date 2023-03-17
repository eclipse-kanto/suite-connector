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
	"os"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
)

const (
	// TopicsEvent defines event topics
	TopicsEvent = "event/#,e/#"

	// TopicsTelemetry defines telemetry topics
	TopicsTelemetry = "telemetry/#,t/#"
)

type eventsHandler struct {
	prefix   string
	tenantID string
	deviceID string
}

// NewEventsHandler returns the events handler function.
func NewEventsHandler(prefix string, tenantID string, deviceID string) message.HandlerFunc {
	h := &eventsHandler{
		prefix:   prefix,
		tenantID: tenantID,
		deviceID: deviceID,
	}

	return h.HandleEvent
}

func (h *eventsHandler) HandleEvent(msg *message.Message) ([]*message.Message, error) {
	if topic, ok := connector.TopicFromCtx(msg.Context()); ok {
		msgType := "event"
		tenID := h.tenantID
		devID := h.deviceID

		segments := strings.Split(topic, "/")

		if segments[0] == "telemetry" || segments[0] == "t" {
			msgType = "telemetry"
		}

		if len(segments) > 1 && len(segments[1]) > 0 {
			tenID = segments[1]
		}

		if len(segments) > 2 && len(segments[2]) > 0 {
			devID = segments[2]
		}

		var buff strings.Builder
		buff.Grow(64)

		if len(h.prefix) > 0 {
			buff.WriteString(h.prefix)
			buff.WriteString("/")
		}

		buff.WriteString(msgType)
		buff.WriteString("/")
		buff.WriteString(tenID)
		buff.WriteString("/")
		buff.WriteString(devID)

		//Fix missing suffix
		for i := 3; i < len(segments); i++ {
			buff.WriteString("/")
			buff.WriteString(segments[i])
		}

		msg.SetContext(connector.SetTopicToCtx(msg.Context(), buff.String()))

		return []*message.Message{msg}, nil
	}
	return nil, errNoTopic
}

// EventsBus creates bus for events messages.
func EventsBus(router *message.Router,
	honoPub message.Publisher,
	mosquittoSub message.Subscriber,
	tenantID string,
	deviceID string,
) *message.Handler {
	prefix := os.Getenv("EVENTS_TOPIC_PREFIX")

	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("events_bus",
		TopicsEvent,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewEventsHandler(prefix, tenantID, deviceID),
	)
}

// TelemetryBus creates bus for telemetry messages.
func TelemetryBus(router *message.Router,
	honoPub message.Publisher,
	mosquittoSub message.Subscriber,
	tenantID string,
	deviceID string,
) *message.Handler {
	prefix := os.Getenv("TELEMETRY_TOPIC_PREFIX")

	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("telemetry_bus",
		TopicsTelemetry,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		NewEventsHandler(prefix, tenantID, deviceID),
	)
}
