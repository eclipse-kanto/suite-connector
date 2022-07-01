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

package testutil_test

import (
	"testing"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type logFacadeMock struct {
	mock.Mock
}

func (m *logFacadeMock) Log(args ...interface{}) {
	m.Called(args...)
}

func TestNewLogger(t *testing.T) {
	f := new(logFacadeMock)
	f.On("Log", "[testutil]", "INFO  ", "Simple message")

	l := testutil.NewLogger("testutil", logger.TRACE, f)
	assert.NotNil(t, l)
	l.Info("Simple message\n", nil)

	f.AssertExpectations(t)
}
