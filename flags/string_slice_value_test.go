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

package flags_test

import (
	"flag"
	"io"
	"testing"

	"github.com/eclipse-kanto/suite-connector/flags"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringSliceVIsSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	var s []string
	v := flags.NewStringSliceV(&s)
	f.Var(v, "S", "S")

	args := []string{"-S=a b"}
	require.NoError(t, f.Parse(args))

	assert.True(t, v.Provided())
	assert.Equal(t, "a b", v.String())

	expected := []string{"a", "b"}
	assert.Equal(t, expected, s)
	assert.Equal(t, expected, v.Get())
}

func TestStringSliceVInvalid(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	f.SetOutput(io.Discard)

	var s []string
	v := flags.NewStringSliceV(&s)
	f.Var(v, "S", "S")

	args := []string{"-S="}
	require.Error(t, f.Parse(args))
}
