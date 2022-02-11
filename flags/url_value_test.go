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

	"github.com/eclipse-kanto/suite-connector/flags"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLvIsSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var u string
	v := flags.NewURLV(&u, "tcp://localhost:1883")
	f.Var(v, "U", "U")

	args := []string{"-U=tcp://localhost:8883"}
	require.NoError(t, f.Parse(args))

	assert.True(t, v.Provided())
	assert.Equal(t, "tcp://localhost:8883", v.String())
	assert.Equal(t, "tcp://localhost:8883", u)
}

func TestURLVNotSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var u string
	v := flags.NewURLV(&u, "tcp://localhost:1883")
	f.Var(v, "U", "U")

	args := make([]string, 0)
	require.NoError(t, f.Parse(args))

	assert.False(t, v.Provided())
	assert.Equal(t, "tcp://localhost:1883", v.String())
	assert.Equal(t, 0, len(u))
}

func TestURLVCannotParse(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var u string
	v := flags.NewURLV(&u, "tcp://localhost:1883")
	f.Var(v, "U", "U")

	args := []string{"-U=U"}
	require.Error(t, f.Parse(args))
}
