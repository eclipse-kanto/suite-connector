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
	"io"
	"log"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
)

func BenchmarkTracing(b *testing.B) {
	data := `{
		"topic": "org.eclipse.kanto/test/things/twin/commands/modify",
		"headers": {},
		"path": "/attributes",
		"value": {
		  "test": {
			"package": "connector",
			"version": 1.0
		  }
		}
	  }`

	logger := logger.NewLogger(log.New(io.Discard, "[trace] ", log.Lmsgprefix), logger.TRACE)
	tracer := connector.NewTrace(logger, []string{"c/simple/"})

	h := tracer(message.PassthroughHandler)

	msg := message.NewMessage("tracer-test", []byte(data))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "c/simple/test"))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := h(msg)
		assert.NoError(b, err)
	}
}
