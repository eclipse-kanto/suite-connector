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

package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixCommand(t *testing.T) {
	topic := "command/testTenant/ns:testGateway/testDevice/req//toggle"
	topic, err := fixCommand(topic)
	assert.NoError(t, err)
	assert.Equal(t, "command//ns:testGateway:testDevice/req//toggle", topic)
}

func TestFixCommandError(t *testing.T) {
	topic := "command/testTenant/"
	_, err := fixCommand(topic)
	assert.Error(t, err)
}

func TestFixCommandEmptyDevice(t *testing.T) {
	topic := "command/testTenant/ns:testGateway//req//toggle"
	topic, err := fixCommand(topic)
	assert.NoError(t, err)
	assert.Equal(t, "command///req//toggle", topic)
}

func TestStripGatwayID(t *testing.T) {
	deviceID, err := stripGatewayID("ns:testGateway", "ns:testGateway")
	assert.NoError(t, err)
	assert.Equal(t, "", deviceID)

	deviceID, err = stripGatewayID("ns:testGateway", "ns:testGateway:")
	assert.NoError(t, err)
	assert.Equal(t, "", deviceID)

	deviceID, err = stripGatewayID("ns:testGateway", "ns:testGateway:a")
	assert.NoError(t, err)
	assert.Equal(t, "a", deviceID)

	deviceID, err = stripGatewayID("ns:testGateway", "ns:testGateway:testDevice")
	assert.NoError(t, err)
	assert.Equal(t, "testDevice", deviceID)

	deviceID, err = stripGatewayID("testGateway", "ns:testGateway:testDevice")
	assert.Error(t, err)
}
