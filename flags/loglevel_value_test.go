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

package flags_test

import (
	"flag"
	"testing"

	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/eclipse-kanto/suite-connector/flags"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogLevelVSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var s logger.LogLevel
	v := flags.NewLogLevelV(&s, logger.INFO)
	f.Var(v, "l", "l")

	args := []string{"-l=DEBUG"}
	require.NoError(t, f.Parse(args))

	assert.True(t, v.Provided())
	assert.Equal(t, "DEBUG", v.String())
	assert.Equal(t, logger.DEBUG, s)
}

func TestLogLevelVNotSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var s logger.LogLevel
	v := flags.NewLogLevelV(&s, logger.INFO)
	f.Var(v, "l", "l")

	args := make([]string, 0)
	require.NoError(t, f.Parse(args))

	assert.False(t, v.Provided())
	assert.Equal(t, "INFO", v.String())
}

func TestLogLevelVInvalid(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var s logger.LogLevel
	v := flags.NewLogLevelV(&s, logger.INFO)
	f.Var(v, "l", "l")

	args := []string{"-l="}
	require.NoError(t, f.Parse(args))
	assert.Equal(t, logger.ERROR, s)
}
