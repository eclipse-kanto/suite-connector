// Copyright (c) 2022 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0
//
// SPDX-License-Identifier: EPL-2.0

package connector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapper(t *testing.T) {
	config, err := NewMQTTClientConfig("tcp://testing:1883")
	require.NoError(t, err)

	w := newWrapper(config, nil, nil)
	require.NotNil(t, w)

	assert.NoError(t, w.Close())
	assert.NoError(t, w.Close()) //Close twice verification
}
