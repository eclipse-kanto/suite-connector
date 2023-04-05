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

package config

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/tpmtls"
)

// Cleaner type is used to define the clean-up function.
type Cleaner func()

func noClean() {
	//Nothing to cleanup
}

// TLSSettings represents the TLS configuration data.
type TLSSettings struct {
	CACert string `json:"caCert"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
	Alpn   string `json:"alpn"`

	TPMKey    string `json:"tpmKey"`
	TPMKeyPub string `json:"tpmKeyPub"`
	TPMHandle uint64 `json:"tpmHandle"`
	TPMDevice string `json:"tpmDevice"`
}

// NewHubTLSConfig initializes the Hub TLS.
func NewHubTLSConfig(settings *TLSSettings, logger watermill.LoggerAdapter) (*tls.Config, Cleaner, error) {
	cfg, clean, err := newHubTLSConfig0(settings, logger)
	if err != nil {
		return nil, clean, err
	}

	if len(settings.Alpn) > 0 {
		cfg.NextProtos = []string{settings.Alpn}
	}

	return cfg, clean, err
}

// NewLocalTLSConfig initializes the Local broker TLS.
func NewLocalTLSConfig(settings *LocalConnectionSettings, logger watermill.LoggerAdapter) (*tls.Config, error) {
	caCertPool, err := NewCAPool(settings.LocalCACert)
	if err != nil {
		return nil, err
	}

	if len(settings.LocalCert) > 0 || len(settings.LocalKey) > 0 {
		return NewFSTlsConfig(caCertPool, settings.LocalCert, settings.LocalKey)
	}

	cfg := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites:       supportedCipherSuites(),
	}

	return cfg, nil
}

// NewCAPool opens a certificates pool.
func NewCAPool(caFile string) (*x509.CertPool, error) {
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load CA")
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.Errorf("failed to parse CA %s", caFile)
	}

	return caCertPool, nil
}

// NewFSTlsConfig initializes a file Hub TLS.
func NewFSTlsConfig(caCertPool *x509.CertPool, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load X509 key pair")
	}

	cfg := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites:       supportedCipherSuites(),
	}

	return cfg, nil
}

// NewTPMTlsConfig initializes s TPM Hub TLS.
func NewTPMTlsConfig(
	settings *TLSSettings,
	caCertPool *x509.CertPool,
	logger watermill.LoggerAdapter,
) (*tls.Config, Cleaner, error) {
	conn, err := tpmtls.NewTpmConnection(settings.TPMDevice)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open TPM descriptor")
	}

	rw, err := conn.GetRW()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open TPM descriptor")
	}

	tpmConfig := &tpmtls.ContextOpts{
		PrivateKeyFile:       settings.TPMKey,
		PublicKeyFile:        settings.TPMKeyPub,
		PublicCertFile:       settings.Cert,
		StorageRootKeyHandle: uint32(settings.TPMHandle),
		TPMConnectionRW:      rw,
		ExtTLSConfig: &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
		},
	}

	tpm, err := tpmtls.NewTPMContext(tpmConfig, logger)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get TPM context")
	}

	tlsConfig := tpm.TLSConfig()
	if tlsConfig == nil {
		return nil, nil, errors.New("TPM TLS backend not working")
	}

	closer := func() {
		rw, err := tpm.Close()
		if err == nil {
			_ = conn.ReleaseRW(rw)
		}
		_ = conn.Close()
	}

	return tlsConfig, closer, nil
}

func newHubTLSConfig0(settings *TLSSettings, logger watermill.LoggerAdapter) (*tls.Config, Cleaner, error) {
	caCertPool, err := NewCAPool(settings.CACert)
	if err != nil {
		return nil, nil, err
	}

	if len(settings.TPMDevice) > 0 {
		return NewTPMTlsConfig(settings, caCertPool, logger)
	}

	if len(settings.Cert) > 0 || len(settings.Key) > 0 {
		cfg, err := NewFSTlsConfig(caCertPool, settings.Cert, settings.Key)
		return cfg, noClean, err
	}

	cfg := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            caCertPool,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites:       supportedCipherSuites(),
	}

	return cfg, noClean, nil
}

func supportedCipherSuites() []uint16 {
	cs := tls.CipherSuites()
	cid := make([]uint16, len(cs))
	for i := range cs {
		cid[i] = cs[i].ID
	}
	return cid
}
