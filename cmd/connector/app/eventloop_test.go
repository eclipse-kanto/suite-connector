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

package app

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"go.uber.org/goleak"

	"github.com/fsnotify/fsnotify"

	"github.com/stretchr/testify/assert"

	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/eclipse-kanto/suite-connector/testutil"
)

func TestMockLauncher(t *testing.T) {
	var l Launcher = NewMockLauncher(nil, nil, nil)
	assert.NotNil(t, l)
	assert.NoError(t, l.Run(false, nil, nil, nil))
}

func TestStopLauncher(t *testing.T) {
	assert.NotPanics(t, func() {
		var l Launcher
		stopLauncher(l)
	})
	assert.NotPanics(t, func() {
		l := (*mockLauncher)(nil)
		stopLauncher(l)
	})
	assert.NotPanics(t, func() {
		l := new(mockLauncher)
		stopLauncher(l)
	})
}

func TestEventLoop(t *testing.T) {
	const name = "test"

	defer goleak.VerifyNone(t)

	watchEvents := make(chan fsnotify.Event, 5)

	watchEvents <- fsnotify.Event{
		Name: name,
		Op:   fsnotify.Create,
	}
	watchEvents <- fsnotify.Event{
		Name: name,
		Op:   fsnotify.Write,
	}
	watchEvents <- fsnotify.Event{
		Name: name,
		Op:   fsnotify.Chmod,
	}
	watchEvents <- fsnotify.Event{
		Name: name,
		Op:   fsnotify.Remove,
	}
	watchEvents <- fsnotify.Event{
		Name: name,
		Op:   fsnotify.Rename,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var launched bool
	launch := func(e fsnotify.Event) {
		launched = true
	}

	EventLoop(ctx, watchEvents, name, launch)
	assert.True(t, launched)

	assert.NotPanics(t, func() {
		EventLoop(ctx, nil, name, launch)
	})
}

func TestMessagesRouter(t *testing.T) {
	logger := testutil.NewLogger("testing", logger.ERROR, t)

	router := NewRouter(logger)

	sub := gochannel.NewGoChannel(
		gochannel.Config{
			OutputChannelBuffer: int64(1),
		},
		logger,
	)

	router.AddNoPublisherHandler(
		"dummy",
		"dummy",
		sub,
		func(msg *message.Message) error {
			return nil
		},
	)

	StartRouter(router)
	time.Sleep(2 * time.Second)
	StopRouter(router)
}
