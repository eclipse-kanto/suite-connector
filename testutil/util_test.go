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

package testutil_test

import (
	"testing"

	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalConfig(t *testing.T) {
	conf, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	assert.NotNil(t, conf)
}

func TestNewLocalConfigInvalidURI(t *testing.T) {
	t.Setenv("TEST_MQTT_URI", "invalid")

	conf, err := testutil.NewLocalConfig()
	assert.Error(t, err)
	assert.Nil(t, conf)
}
