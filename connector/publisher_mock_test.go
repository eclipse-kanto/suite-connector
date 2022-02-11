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
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/connector"
)

func TestMockPublishClose(t *testing.T) {
	pub := connector.NewErrPublisher(nil)
	pub.Close()
	assert.ErrorIs(t, connector.ErrClosed, pub.Close())
	assert.ErrorIs(t, connector.ErrClosed, pub.Publish("e1", message.NewMessage("e1", []byte("e1"))))
}

func TestMockPublishError(t *testing.T) {
	pub := connector.NewErrPublisher(errors.New("simple failure"))
	assert.Error(t, pub.Publish("simple/error", message.NewMessage("e1", []byte("e1"))))
}

func TestNullPublisher(t *testing.T) {
	pub := connector.NullPublisher()
	assert.NoError(t, pub.Publish("null/event", message.NewMessage("null", []byte("null"))))
	assert.NoError(t, pub.Close())
}
