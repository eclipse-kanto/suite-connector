// Copyright (c) 2023 Contributors to the Eclipse Foundation
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

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkTimestamp(b *testing.B) {
	data := `{
		"topic": "org.eclipse.kanto/test/things/twin/commands/modify",
		"headers": {
			"correlation-id": "59a8a9f9-c8c6-41f9-a5d7-a6c00fd304cd",
			"response-required": false
		},
		"path": "/features/meter",
		"value": {
			"properties": {
				"x": 12.34,
				"y": 5.6
			}
		}
	}`

	h := routing.AddTimestamp(message.PassthroughHandler)

	msg := message.NewMessage("test", []byte(data))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "event/testTenant/testDevice"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := h(msg)
		assert.NoError(b, err)
	}
}
