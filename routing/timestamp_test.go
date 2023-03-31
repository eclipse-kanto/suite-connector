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
	"encoding/json"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"

	"github.com/eclipse/ditto-clients-golang/protocol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTimestamp(t *testing.T) {
	h := routing.AddTimestamp(message.PassthroughHandler)

	msg := message.NewMessage("test", []byte("{}"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "event/testTenant/testDevice"))

	msgs, err := h(msg)
	require.NoError(t, err)
	require.Equal(t, 1, len(msgs))

	env := new(protocol.Envelope)
	require.NoError(t, json.Unmarshal(msgs[0].Payload, &env))

	ts := env.Headers.Generic("x-timestamp")
	require.NotNil(t, ts)
	timestamp, ok := ts.(float64)
	require.True(t, ok)
	assert.Greater(t, timestamp, 0.0)
}
