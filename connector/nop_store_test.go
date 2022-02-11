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

package connector

import (
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/stretchr/testify/assert"
)

func TestNopStore(t *testing.T) {
	store := new(nostore)
	store.Open()
	store.Reset()

	defer store.Close()

	m := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	m.Qos = 1
	m.TopicName = "/testing"
	m.Payload = []byte{0x12, 0x34}
	m.MessageID = 1234

	store.Put("test", m)
	defer store.Del("test")

	assert.Nil(t, store.Get("test"))
	assert.Equal(t, 0, len(store.All()))
}
