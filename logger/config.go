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

package logger

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"

	"gopkg.in/natefinch/lumberjack.v2"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	logFlags int = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lmsgprefix
)

func init() {
	log.SetFlags(logFlags)
}

// LogSettings defines the Logger configuration.
type LogSettings struct {
	LogFile       string   `json:"logFile"`
	LogLevel      LogLevel `json:"logLevel"`
	LogFileSize   int      `json:"logFileSize"`
	LogFileCount  int      `json:"logFileCount"`
	LogFileMaxAge int      `json:"logFileMaxAge"`
}

// Validate validates the Logger settings.
func (c *LogSettings) Validate() error {
	if c.LogFileSize <= 0 {
		return errors.New("logFileSize <= 0")
	}

	if c.LogFileCount <= 0 {
		return errors.New("logFileCount <= 0")
	}

	if c.LogFileMaxAge < 0 {
		return errors.New("logFileMaxAge < 0")
	}

	return nil
}

// ConfigMQTT sets level for paho MQTT logging
func ConfigMQTT(level LogLevel, logger *log.Logger) {
	mqtt.ERROR = NewLevelDecorator(logger, ERROR)
	mqtt.CRITICAL = NewLevelDecorator(logger, ERROR)

	if level >= WARN {
		mqtt.WARN = NewLevelDecorator(logger, WARN)
	}

	if level >= DEBUG {
		mqtt.DEBUG = NewLevelDecorator(logger, DEBUG)
	}
}

// Setup creates the logger instance with provided name and settings.
func Setup(name string, settings *LogSettings) (io.WriteCloser, Logger) {
	loggerOut := newLoggerOut(settings)

	pahoLogLevel := os.Getenv("MQTT_LOG_LEVEL")
	if len(pahoLogLevel) > 0 {
		pahoLog := create(loggerOut, "paho", len(name))
		ConfigMQTT(ParseLogLevel(pahoLogLevel), pahoLog)
	}

	rootLog := create(loggerOut, name, len(name))
	rootLogger := NewLogger(rootLog, settings.LogLevel)

	return loggerOut, rootLogger
}

func create(out io.Writer, prefix string, prefixLen int) *log.Logger {
	format := fmt.Sprintf(" %%-%vs", prefixLen+4)
	return log.New(out, fmt.Sprintf(format, "["+prefix+"]"), logFlags)
}

type nopWriterCloser struct {
	out io.Writer
}

// Write prints the data.
func (w *nopWriterCloser) Write(p []byte) (n int, err error) {
	return w.out.Write(p)
}

// Close should not be used for NOP writer.
func (*nopWriterCloser) Close() error {
	return nil
}

func newNopWriterCloser(out io.Writer) io.WriteCloser {
	return &nopWriterCloser{out: out}
}

func newLoggerOut(settings *LogSettings) io.WriteCloser {
	if len(settings.LogFile) == 0 {
		return newNopWriterCloser(os.Stderr)
	}
	return &lumberjack.Logger{
		Filename:   settings.LogFile,
		MaxSize:    settings.LogFileSize,
		MaxBackups: settings.LogFileCount,
		MaxAge:     settings.LogFileMaxAge,
		LocalTime:  true,
		Compress:   true,
	}
}
