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

package util_test

import (
	"testing"
	"time"

	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/util"
)

func TestNormalizing(t *testing.T) {
	message := util.NormalizeTopic("c//4711 /q/#")
	assert.Equal(t, message, "command//4711 /req/#")

	message = util.NormalizeTopic("c///q/#")
	assert.Equal(t, message, "command///req/#")

	message = util.NormalizeTopic("command//4711 /req/#")
	assert.Equal(t, message, "command//4711 /req/#")
}

func TestParseCmdTopic(t *testing.T) {
	cmdType, prefix, suffix := util.ParseCmdTopic("topic")
	assert.EqualValues(t, "", cmdType)
	assert.EqualValues(t, "", prefix)
	assert.EqualValues(t, "", suffix)

	cmdType, prefix, suffix = util.ParseCmdTopic("c//sPrefix/s/sSuffix")
	assert.EqualValues(t, "s", cmdType)
	assert.EqualValues(t, "sPrefix", prefix)
	assert.EqualValues(t, "sSuffix", suffix)

	cmdType, prefix, suffix = util.ParseCmdTopic("command//commandPrefix/res/commandSuffix")
	assert.EqualValues(t, "res", cmdType)
	assert.EqualValues(t, "commandPrefix", prefix)
	assert.EqualValues(t, "commandSuffix", suffix)

	cmdType, prefix, suffix = util.ParseCmdTopic("command//commandPrefix/req/request-id/command")
	assert.EqualValues(t, "req", cmdType)
	assert.EqualValues(t, "commandPrefix", prefix)
	assert.EqualValues(t, "request-id/command", suffix)
}

func TestFileExists(t *testing.T) {
	assert.False(t, util.FileExists(""))
	assert.False(t, util.FileExists("NonExisting.txt"))
	assert.False(t, util.FileExists("../util"))

	assert.True(t, util.FileExists("../util/utils_test.go"))
}

func TestValidPath(t *testing.T) {
	require.Error(t, util.ValidPath(""))

	require.NoError(t, util.ValidPath("NonExisting.txt"))
	require.NoError(t, util.ValidPath("../config"))
	require.NoError(t, util.ValidPath("../config/utils.go"))
}

func TestResponseStatusTopic(t *testing.T) {
	rspTopic := util.ResponseStatusTopic("command///req//response", 500)
	assert.Equal(t, "command///res//500", rspTopic)

	rspTopic = util.ResponseStatusTopic("command///req/test-id/response", 204)
	assert.Equal(t, "command///res/test-id/204", rspTopic)

	rspTopic = util.ResponseStatusTopic("command//device-id/req/test-id/response", 400)
	assert.Equal(t, "command//device-id/res/test-id/400", rspTopic)
}

func TestHonoUserName(t *testing.T) {
	assert.Equal(t, "testAuth@testTenant", util.NewHonoUserName("testAuth", "testTenant"))
}

func TestTimeoutParse(t *testing.T) {
	headers := protocol.WithTimeout("10s")
	assert.Equal(t, 10*time.Second, util.ParseTimeout(protocol.NewHeaders(headers).Timeout()))

	headers = protocol.WithTimeout("500ms")
	assert.Equal(t, time.Millisecond*500, util.ParseTimeout(protocol.NewHeaders(headers).Timeout()))

	headers = protocol.WithTimeout("1m")
	assert.Equal(t, time.Minute, util.ParseTimeout(protocol.NewHeaders(headers).Timeout()))

	headers = protocol.WithTimeout("0")
	assert.Equal(t, 0*time.Second, util.ParseTimeout(protocol.NewHeaders(headers).Timeout()))

	headers = protocol.WithTimeout("")
	assert.Equal(t, 60*time.Second, util.ParseTimeout(protocol.NewHeaders(headers).Timeout()))
}
