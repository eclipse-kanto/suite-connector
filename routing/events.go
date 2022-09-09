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
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
)

const (
	// TopicsEvent defines events and telemetry topics
	TopicsEvent = "event/#,e/#,telemetry/#,t/#"

	// TopicsTelemetry defines telemetry topics
	TopicsTelemetry = "telemetry/#,t/#"
)

// EventsBus creates bus for events and telemetry messages.
func EventsBus(router *message.Router, honoPub message.Publisher, mosquittoSub message.Subscriber) *message.Handler {
	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("events_bus",
		TopicsEvent,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		message.PassthroughHandler,
	)
}

// TelemetryBus creates bus for telemetry messages.
func TelemetryBus(router *message.Router, honoPub message.Publisher, mosquittoSub message.Subscriber) *message.Handler {
	//Gateway -> Mosquitto Broker -> Message bus -> Hono
	return router.AddHandler("telemetry_bus",
		TopicsTelemetry,
		mosquittoSub,
		connector.TopicEmpty,
		honoPub,
		message.PassthroughHandler,
	)
}
