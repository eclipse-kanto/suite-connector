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

package connector_test

import (
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

func TestMarshaller(t *testing.T) {
	m := conn.NewDefaultMarshaller()

	payload := make([]byte, 10)
	_, err := rand.Read(payload)
	assert.NoError(t, err)

	var mid uint16 = 123
	msg, err := m.Unmarshal(mid, payload)
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(mid), 10), msg.UUID)

	b, err := m.Marshal(msg)
	assert.NoError(t, err)
	assert.Equal(t, payload, b)
}
