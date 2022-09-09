// Copyright (c) 2022 Contributors to the Eclipse Foundation
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

package connector

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebugEnable(t *testing.T) {
	l := logger.NewLogger(log.New(io.Discard, "[connector]", log.Ldate), logger.DEBUG)
	assert.True(t, isDebugEnabled(l))

	l1 := watermill.NewStdLogger(true, false)
	assert.True(t, isDebugEnabled(l1))
}

func TestParseSubs(t *testing.T) {
	topics := parseSubs("a/#,a/+/b")
	require.NotNil(t, topics)
	assert.Equal(t, []string{"a/#", "a/+/b"}, topics)

	filters := subFilters(QosAtLeastOnce, topics)
	assert.Contains(t, filters, "a/#")
	assert.Contains(t, filters, "a/+/b")
}

func TestMessageTimeout(t *testing.T) {
	timeout := 10 * time.Second
	assert.Greater(t, timeout, calcTimeout(timeout))
}
