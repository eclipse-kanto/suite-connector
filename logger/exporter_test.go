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

package logger

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStdExporter(t *testing.T) {
	b := new(bytes.Buffer)
	e := newStdExporter(log.New(b, "[exporter] ", log.Lmsgprefix))

	e.Export(DEBUG, "exporter")
	assert.Contains(t, b.String(), "exporter")
}
