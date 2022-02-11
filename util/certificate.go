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

package util

import (
	"crypto/tls"
	"crypto/x509"
	"strings"

	"github.com/pkg/errors"
)

const (
	cn = "{{subject-cn}}"
	dn = "{{subject-dn}}"
)

// ReplacePattern resolves a pettern using the provided certificate data.
func ReplacePattern(pattern string, cert tls.Certificate) (string, error) {
	if len(cert.Certificate) == 0 {
		return "", errors.New("failed to load Subject and Common Name from certificates file")
	}

	parseCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return "", err
	}

	index := strings.Index(pattern, dn)
	if index >= 0 {
		pattern = strings.ReplaceAll(pattern, dn, parseCert.Subject.String())
	}

	index = strings.Index(pattern, cn)
	if index >= 0 {
		pattern = strings.ReplaceAll(pattern, cn, parseCert.Subject.CommonName)
	}

	return pattern, nil
}
