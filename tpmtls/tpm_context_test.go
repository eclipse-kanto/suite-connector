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

package tpmtls

import (
	"bytes"
	"crypto/tls"
	"strings"
	"testing"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testKey  = "testdata/tpm-key.pem"
	testCert = "testdata/tpm-cert.pem"
)

func TestNoTPMConnection(t *testing.T) {
	assertCreateTPMContextError(t, new(ContextOpts), "the TPM connection stream is required")
}

func TestTPMNoStorageRootKeyHandle(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW: new(bytes.Buffer),
	}
	assertCreateTPMContextError(t, ctxOpts, "the TPM Storage Root Key handle is required")
}

func TestTPMNoPublicKeyFile(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
	}
	assertCreateTPMContextError(t, ctxOpts, "the TPM Public Key File is required")
}

func TestTPMNoPrivateKeyFile(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PublicKeyFile:        testKey,
	}
	assertCreateTPMContextError(t, ctxOpts, "the TPM Private Key File is required")
}

func TestTPMNoPublicCertFile(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
	}
	assertCreateTPMContextError(t, ctxOpts, "the Public Certificate File is required")
}

func TestTPMNoExtTLSConfig(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
		PublicCertFile:       testCert,
	}
	assertCreateTPMContextError(t, ctxOpts, "the External TLS Config is required")
}

func TestTPMHasExtTLSConfigCertificates(t *testing.T) {
	cert := make([][]byte, 1)
	cert[0] = []byte("certificate")
	certs := make([]tls.Certificate, 1)
	certs[0] = tls.Certificate{
		Certificate: cert,
	}
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
		PublicCertFile:       testCert,
		ExtTLSConfig:         &tls.Config{Certificates: certs},
	}
	assertCreateTPMContextError(t, ctxOpts, "Certificates value in ExtTLSConfig MUST be empty")
}

func TestTPMHasExtTLSConfigCipherSuites(t *testing.T) {
	suites := make([]uint16, 1)
	suites[0] = uint16(1)
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
		PublicCertFile:       testCert,
		ExtTLSConfig:         &tls.Config{CipherSuites: suites},
	}
	assertCreateTPMContextError(t, ctxOpts, "CipherSuites value in ExtTLSConfig MUST be empty")
}

func TestTPMValidateError(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
		PublicCertFile:       "test",
		ExtTLSConfig:         &tls.Config{},
	}
	_, err := NewTPMContext(ctxOpts, testutil.NewLogger("[tpmtls] ", logger.DEBUG, t))
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "unable to read certificate file content: open test: "), err.Error())
}

func TestTPMPubKeyParseError(t *testing.T) {
	ctxOpts := &ContextOpts{
		TPMConnectionRW:      new(bytes.Buffer),
		StorageRootKeyHandle: 1,
		PrivateKeyFile:       testKey,
		PublicKeyFile:        testKey,
		PublicCertFile:       testCert,
		ExtTLSConfig:         &tls.Config{},
	}
	_, err := NewTPMContext(ctxOpts, testutil.NewLogger("[tpmtls] ", logger.DEBUG, t))
	require.Error(t, err)
	assert.Equal(t, "unexpected EOF", err.Error())
}

func assertCreateTPMContextError(t *testing.T, opts *ContextOpts, expectedErr string) {
	_, err := NewTPMContext(opts, nil)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err.Error())
}
