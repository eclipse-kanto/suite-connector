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
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"go.uber.org/goleak"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

func TestNilPublisher(t *testing.T) {
	defer goleak.VerifyNone(t)

	assert.Panics(t, func() {
		conn.NewRateLimiter(nil, 20, time.Second)
	}, "No publisher panic")
}

func TestClosed(t *testing.T) {
	defer goleak.VerifyNone(t)

	pubSub := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(64)},
		watermill.NopLogger{},
	)

	ratelimit := conn.NewRateLimiter(pubSub, 20, time.Second)
	assert.NoError(t, ratelimit.Publish("test", message.NewMessage("test", []byte("test"))))
	assert.NoError(t, ratelimit.Close())
	assert.NoError(t, ratelimit.Close()) //Second close by accident
	assert.Error(t, ratelimit.Publish("test", message.NewMessage("test", []byte("test"))))
}

func TestMessageCancel(t *testing.T) {
	defer goleak.VerifyNone(t)

	pubSub := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: int64(4)},
		watermill.NopLogger{},
	)

	ratelimit := conn.NewRateLimiter(pubSub, 10, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	m := message.NewMessage("test", []byte("test"))
	m.SetContext(ctx)

	_ = ratelimit.Publish("test", m)
	assert.Error(t, ratelimit.Publish("test", m))
}

func TestRateLimitWithError(t *testing.T) {
	defer goleak.VerifyNone(t)

	pubSub := conn.NewErrPublisher(errors.New("simple failure"))
	ratelimit := conn.NewRateLimiter(pubSub, 10, time.Millisecond)
	defer ratelimit.Close()

	var msgs []*message.Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, message.NewMessage("test", []byte("test")))
	}

	assert.Error(t, ratelimit.Publish("test", msgs...))
}
