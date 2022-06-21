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

import "log"

// Exporter interface allows customization of log target.
type Exporter interface {
	Export(level LogLevel, msg string)
}

type stdExporter struct {
	logger *log.Logger
}

func newStdExporter(logger *log.Logger) Exporter {
	return &stdExporter{logger: logger}
}

func (e *stdExporter) Export(level LogLevel, msg string) {
	if l := len(msg); l > 0 && msg[l-1] == '\n' {
		msg = msg[0 : l-1]
	}
	e.logger.Println(level.StringAligned(), msg)
}
