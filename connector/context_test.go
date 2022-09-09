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

package connector_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

func TestContextQos(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	qos, ok := conn.QosFromCtx(ctx)
	require.False(t, ok)
	require.Equal(t, conn.QosAtMostOnce, qos)

	ctx = conn.SetQosToCtx(ctx, conn.QosAtMostOnce)
	qos, ok = conn.QosFromCtx(ctx)
	require.True(t, ok)
	require.Equal(t, conn.QosAtMostOnce, qos)

	ctx = conn.SetQosToCtx(ctx, conn.QosAtLeastOnce)
	qos, ok = conn.QosFromCtx(ctx)
	require.True(t, ok)
	require.Equal(t, conn.QosAtLeastOnce, qos)

	ctx = conn.SetQosToCtx(ctx, conn.QosAtMostOnce)
	qos, ok = conn.QosFromCtx(ctx)
	require.True(t, ok)
	require.Equal(t, conn.QosAtMostOnce, qos)
}

func TestContextRetain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = conn.SetRetainToCtx(ctx, false)
	assert.False(t, conn.RetainFromCtx(ctx))

	ctx = conn.SetRetainToCtx(ctx, true)
	assert.True(t, conn.RetainFromCtx(ctx))

	ctx = conn.SetRetainToCtx(ctx, false)
	assert.False(t, conn.RetainFromCtx(ctx))

	ctx = conn.SetRetainToCtx(ctx, true)
	assert.True(t, conn.RetainFromCtx(ctx))
}

func TestContextTopic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = conn.SetTopicToCtx(ctx, "simple/test")
	topic, ok := conn.TopicFromCtx(ctx)
	assert.True(t, ok)
	assert.Equal(t, "simple/test", topic)
}
