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

package testutil

import (
	"fmt"
	"log"
	"os"

	"github.com/eclipse-kanto/suite-connector/logger"
)

const (
	logFlags int = log.LstdFlags | log.Lmicroseconds | log.Lshortfile
)

// LogFacade interface allows access to runtime logger
type LogFacade interface {
	Log(args ...interface{})
}

type testexporter struct {
	f    LogFacade
	name string
}

func (e *testexporter) Export(level logger.LogLevel, msg string) {
	if l := len(msg); l > 0 && msg[l-1] == '\n' {
		msg = msg[0 : l-1]
	}
	e.f.Log(e.name, level.StringAligned(), msg)
}

func newTestExporter(name string, f LogFacade) logger.Exporter {
	return &testexporter{
		f:    f,
		name: fmt.Sprintf("[%s]", name),
	}
}

// NewLogger returns test runtime logger
func NewLogger(name string, level logger.LogLevel, f LogFacade) logger.Logger {
	return logger.NewLoggerWithExporter(newTestExporter(name, f), level)
}

func init() {
	if pahoLogLevel := os.Getenv("TEST_MQTT_LOG_LEVEL"); len(pahoLogLevel) > 0 {
		pahoLog := log.New(os.Stdout, fmt.Sprintf("[%s]", "paho"), logFlags)
		logger.ConfigMQTT(logger.ParseLogLevel(pahoLogLevel), pahoLog)
	}
}
