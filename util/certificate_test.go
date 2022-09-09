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
	"crypto/tls"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/util"
)

func TestParseDeviceIDPattern(t *testing.T) {
	certFile := "testdata/certificate.pem"
	keyFile := "testdata/key.pem"

	type patternTest struct {
		pattern  string
		expected string
	}
	prefix := "org.eclipse.kanto:"
	suffix := ":{{subject-dn1}}"

	tests := []patternTest{
		{
			pattern: "{{subject-dn}}",
			expected: "CN=www.example-inc.com,OU=HR Department,O=Example Inc.," +
				"L=Boston,ST=Massachusetts,C=US",
		},
		{
			pattern:  "{{subject-cn}}",
			expected: "www.example-inc.com",
		},
		{
			pattern: "",
		},
		{
			pattern: "{{subject-dn}}:{{subject-cn}}",
			expected: "CN=www.example-inc.com,OU=HR Department,O=Example Inc.,L=Boston," +
				"ST=Massachusetts,C=US:www.example-inc.com",
		},
		{
			pattern: "{{subject-{{subject-cn}}dn}}:{{subject-dn}}",
			expected: "{{subject-www.example-inc.comdn}}:" +
				"CN=www.example-inc.com,OU=HR Department,O=Example Inc.,L=Boston," +
				"ST=Massachusetts,C=US",
		},
	}
	for _, test := range tests {
		assertPattern(t, test.pattern, test.expected, certFile, keyFile)
		assertPattern(t, prefix+test.pattern, prefix+test.expected, certFile, keyFile)
		assertPattern(t, test.pattern+suffix, test.expected+suffix, certFile, keyFile)
		assertPattern(t, prefix+test.pattern+suffix, prefix+test.expected+suffix, certFile, keyFile)
		assertPattern(t, test.pattern+":"+test.pattern, test.expected+":"+test.expected, certFile, keyFile)
	}
}

func assertPattern(t *testing.T, pattern, expected, certFile, keyFile string) {
	deviceId, err := resolvePattern(pattern, certFile, keyFile)
	require.NoError(t, err, pattern, certFile, keyFile)
	assert.EqualValues(t, expected, deviceId)
}

func TestParseDeviceIDPatternError(t *testing.T) {
	_, err := resolvePattern("mypattern", "testdata/certificate.pem", "testdata/email_key.pem")
	assert.Error(t, err)
}

func resolvePattern(pattern string, certFile string, keyFile string) (string, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to load X509 key pair")
	}
	return util.ReplacePattern(pattern, cert)
}

func TestReplacePatternEmptyCertError(t *testing.T) {
	_, err := util.ReplacePattern("noCert", tls.Certificate{})
	assert.Error(t, err)

	_, err = util.ReplacePattern("emptyCert", tls.Certificate{
		Certificate: make([][]byte, 1),
	})
	assert.Error(t, err)
}
