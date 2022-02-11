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

package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogLevelReprAndAlignment(t *testing.T) {
	tests := []struct {
		level   LogLevel
		repr    string
		aligned string
	}{
		{ERROR, "ERROR", "ERROR "},
		{WARN, "WARN", "WARN  "},
		{INFO, "INFO", "INFO  "},
		{DEBUG, "DEBUG", "DEBUG "},
		{TRACE, "TRACE", "TRACE "},
		{0, "ERROR", "ERROR "},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.repr, tt.level.String())
		assert.Equal(t, tt.aligned, tt.level.StringAligned())
		if tt.level == 0 {
			assert.Equal(t, tt.level+1, ParseLogLevel(tt.repr))
		} else {
			assert.Equal(t, tt.level, ParseLogLevel(tt.repr))
		}
	}
}

func TestLevelEnable(t *testing.T) {
	lvl := TRACE
	assert.True(t, lvl.Enabled(TRACE))
	assert.True(t, lvl.Enabled(DEBUG))
	assert.True(t, lvl.Enabled(INFO))
	assert.True(t, lvl.Enabled(WARN))
	assert.True(t, lvl.Enabled(ERROR))

	lvl = DEBUG
	assert.False(t, lvl.Enabled(TRACE))
	assert.True(t, lvl.Enabled(DEBUG))
	assert.True(t, lvl.Enabled(INFO))
	assert.True(t, lvl.Enabled(WARN))
	assert.True(t, lvl.Enabled(ERROR))

	lvl = INFO
	assert.False(t, lvl.Enabled(TRACE))
	assert.False(t, lvl.Enabled(DEBUG))
	assert.True(t, lvl.Enabled(INFO))
	assert.True(t, lvl.Enabled(WARN))
	assert.True(t, lvl.Enabled(ERROR))

	lvl = WARN
	assert.False(t, lvl.Enabled(TRACE))
	assert.False(t, lvl.Enabled(DEBUG))
	assert.False(t, lvl.Enabled(INFO))
	assert.True(t, lvl.Enabled(WARN))
	assert.True(t, lvl.Enabled(ERROR))

	lvl = ERROR
	assert.False(t, lvl.Enabled(TRACE))
	assert.False(t, lvl.Enabled(DEBUG))
	assert.False(t, lvl.Enabled(INFO))
	assert.False(t, lvl.Enabled(WARN))
	assert.True(t, lvl.Enabled(ERROR))
}

type Settings struct {
	Level LogLevel `json:"logLevel"`
}

func TestLogLevelSimple(t *testing.T) {
	m := &Settings{
		Level: INFO,
	}

	msgEnc, err := json.Marshal(m)
	require.NoError(t, err)

	var msg Settings
	require.NoError(t, json.Unmarshal(msgEnc, &msg))

	assert.Equal(t, m, &msg)
	assert.Equal(t, INFO, msg.Level)
}

func TestLogLevelUnmarshal(t *testing.T) {
	var msg Settings
	require.NoError(t, json.Unmarshal([]byte(`{"logLevel": "DEBUG"}`), &msg))

	require.Equal(t, DEBUG, msg.Level)
}

func TestLogLevelParseError(t *testing.T) {
	var msg Settings
	assert.Error(t, json.Unmarshal([]byte(`{"logLevel": null}`), &msg))
	assert.Error(t, json.Unmarshal([]byte(`{"logLevel": 4}`), &msg))
	assert.Error(t, json.Unmarshal([]byte(`{"logLevel": true}`), &msg))
	assert.Error(t, json.Unmarshal([]byte(`{"logLevel": ""yes""}`), &msg))
}

func TestLogLevelDecorator(t *testing.T) {
	const msg = "The value is %s\n"
	formatted := fmt.Sprintf(msg, "v")

	b1 := new(bytes.Buffer)
	d := NewLevelDecorator(create(b1, "[testing] ", 15), INFO)

	d.Printf(msg, "v")
	d.Println(formatted)

	output := b1.String()

	assert.True(t, strings.Contains(output, formatted))
}
