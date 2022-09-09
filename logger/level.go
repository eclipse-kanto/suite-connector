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
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
)

// LogLevel defines the logger levels.
type LogLevel int

const (
	// ERROR is used to define the errors logging level.
	ERROR LogLevel = 1 + iota
	// WARN is used to define the errors and warnings logging level.
	WARN
	// INFO is used to define the errors, warnings and information logging level.
	INFO
	// DEBUG is used to define the errors, warnings, information and debug logging level.
	DEBUG
	// TRACE is used to define the more detailed logging level.
	TRACE
)

// Enabled returns true if the level is enabled.
func (l LogLevel) Enabled(level LogLevel) bool {
	return l >= level
}

func (l LogLevel) String() string {
	switch l {
	case WARN:
		return "WARN"
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	case TRACE:
		return "TRACE"
	default:
		return "ERROR"
	}
}

// StringAligned returns the log level as string with padding.
func (l LogLevel) StringAligned() string {
	switch l {
	case WARN:
		return "WARN  "
	case INFO:
		return "INFO  "
	case DEBUG:
		return "DEBUG "
	case TRACE:
		return "TRACE "
	default:
		return "ERROR "
	}
}

// MarshalJSON is used to marshal LogLevel.
func (l LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

// UnmarshalJSON is used to unmarshal LogLevel.
func (l *LogLevel) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	value, ok := v.(string)
	if !ok {
		return errors.Errorf("unrecognized log level: %q", value)
	}

	*l = ParseLogLevel(value)
	return nil
}

// ParseLogLevel returns a LogLevel for a level string representation.
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "WARN":
		return WARN
	case "INFO":
		return INFO
	case "DEBUG":
		return DEBUG
	case "TRACE":
		return TRACE
	default:
		return ERROR
	}
}

// LogLevelDecorator is responsible for printing declaration depending on the level.
type LogLevelDecorator struct {
	exporter Exporter

	level LogLevel
}

// Println prints the values with a new line break.
func (l LogLevelDecorator) Println(v ...interface{}) {
	l.exporter.Export(l.level, fmt.Sprint(v...))
}

// Printf formats the text while printing the data.
func (l LogLevelDecorator) Printf(format string, v ...interface{}) {
	l.exporter.Export(l.level, fmt.Sprintf(format, v...))
}

// NewLevelDecorator creates decorator for given logger and level instances.
func NewLevelDecorator(logger *log.Logger, level LogLevel) LogLevelDecorator {
	return NewLevelDecoratorWithExporter(newStdExporter(logger), level)
}

// NewLevelDecoratorWithExporter creates log level decorator with specified exporter.
func NewLevelDecoratorWithExporter(exporter Exporter, level LogLevel) LogLevelDecorator {
	return LogLevelDecorator{
		exporter: exporter,
		level:    level,
	}
}
