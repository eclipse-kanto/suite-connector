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
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
)

func TestTracing(t *testing.T) {
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

	var buff bytes.Buffer
	require.NoError(t, json.Compact(&buff, []byte(data)))

	capturedLogger := watermill.NewCaptureLogger()
	tracer := connector.NewTrace(capturedLogger, []string{"c/simple/"})

	h := tracer(message.PassthroughHandler)

	msg := message.NewMessage("tracer-test", buff.Bytes())
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "c/simple/test"))

	_, err := h(msg)
	assert.NoError(t, err)

	logFields := watermill.LogFields{
		"message_uuid": "tracer-test",
	}

	assert.True(t, capturedLogger.Has(watermill.CapturedMessage{
		Level:  2,
		Fields: logFields,
		Msg:    buff.String(),
		Err:    nil,
	}))

	if _, err := h(message.NewMessage("tracer-empty", nil)); err != nil {
		assert.NoError(t, err)
	}
}

func TestTracingErrors(t *testing.T) {
	assert.Panics(t, func() { connector.NewTrace(nil, []string{"c/simple/"}) }, "No logger panic")

	logout := log.New(os.Stdout, "[tracing] ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)
	logger := logger.NewLogger(logout, logger.DEBUG)
	tracer := connector.NewTrace(logger, []string{"c/simple/"})
	h := tracer(message.PassthroughHandler)

	msg := message.NewMessage("invalid_json", []byte(`({"id":"test"})`))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), "c/simple/test"))

	_, err := h(msg)
	assert.Error(t, err)
}
