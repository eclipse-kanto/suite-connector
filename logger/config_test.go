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

package logger

import (
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	ErrClosed = errors.New("writer closed")
)

type mockWriter struct {
	closed bool
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, ErrClosed
	}
	return len(p), nil
}

func (w *mockWriter) Close() error {
	if w.closed {
		return ErrClosed
	}
	w.closed = true
	return nil
}

func TestNopWriterCloser(t *testing.T) {
	m := new(mockWriter)
	w := newNopWriterCloser(m)
	w.Close()

	n, err := w.Write([]byte(`test`))
	assert.Equal(t, 4, n)
	assert.NoError(t, err)

	assert.NoError(t, m.Close())
	assert.Error(t, m.Close())
}

func TestLogSettingsValid(t *testing.T) {
	config := &LogSettings{
		LogFileSize:  2,
		LogFileCount: 5,
	}
	assert.NoError(t, config.Validate())
}

func TestLogSettingsInvalid(t *testing.T) {
	config := new(LogSettings)
	assert.Error(t, config.Validate())

	config = &LogSettings{
		LogFileSize: 1,
	}
	assert.Error(t, config.Validate())

	config = &LogSettings{
		LogFileSize:   1,
		LogFileCount:  1,
		LogFileMaxAge: -1,
	}
	assert.Error(t, config.Validate())
}

func TestLogRotateInit(t *testing.T) {
	settings := &LogSettings{
		LogFile:       "testing.log",
		LogFileSize:   1,
		LogFileCount:  1,
		LogFileMaxAge: 1,
	}

	logOut := newLoggerOut(settings)
	require.NotNil(t, logOut)
	require.NoError(t, logOut.Close())

	settings.LogFile = ""
	logOut = newLoggerOut(settings)
	require.NotNil(t, logOut)
	require.NoError(t, logOut.Close())
}

func TestSetupLogger(t *testing.T) {
	if pahoLogLevel := os.Getenv("TEST_MQTT_LOG_LEVEL"); len(pahoLogLevel) > 0 {
		t.Skip("TEST_MQTT_LOG_LEVEL is set")
	}

	t.Setenv("MQTT_LOG_LEVEL", "DEBUG")

	defer func() {
		mqtt.ERROR = mqtt.NOOPLogger{}
		mqtt.CRITICAL = mqtt.NOOPLogger{}
		mqtt.WARN = mqtt.NOOPLogger{}
		mqtt.DEBUG = mqtt.NOOPLogger{}
	}()

	settings := &LogSettings{
		LogLevel:     DEBUG,
		LogFileSize:  2,
		LogFileCount: 5,
	}

	out, logger := Setup("testing", settings)
	require.NotNil(t, out)
	require.NotNil(t, logger)
	defer out.Close()
}
