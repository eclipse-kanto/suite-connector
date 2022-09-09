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

package flags

import (
	"github.com/eclipse-kanto/suite-connector/logger"
)

// LogLevelV represents LogLevel flag value.
type LogLevelV struct {
	setter *logger.LogLevel

	provided bool

	value logger.LogLevel
}

// NewLogLevelV creates new flag variable for LogLevel definition.
func NewLogLevelV(setter *logger.LogLevel, defaultVal logger.LogLevel) *LogLevelV {
	return &LogLevelV{
		setter: setter,
		value:  defaultVal,
	}
}

// Provided returns true if the flag value is provided.
func (f *LogLevelV) Provided() bool {
	return f.provided
}

// String returns the flag string value.
func (f *LogLevelV) String() string {
	return f.value.String()
}

// Get returns the flag converted value.
func (f *LogLevelV) Get() interface{} {
	return f.value
}

// Set validates and applies the provided value if no error.
func (f *LogLevelV) Set(value string) error {
	level := logger.ParseLogLevel(value)

	f.provided = true
	f.value = level
	*f.setter = level

	return nil
}
