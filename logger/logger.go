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

package logger

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
)

// Logger interface enables the loging functionality.
type Logger interface {
	watermill.LoggerAdapter

	IsDebugEnabled() bool

	IsTraceEnabled() bool

	Warn(msg string, err error, fields watermill.LogFields)

	Errorf(format string, a ...interface{})
	Warnf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Debugf(format string, a ...interface{})
	Tracef(format string, a ...interface{})
}

var loggerPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

type loggerext struct {
	logger *log.Logger

	level  LogLevel
	fields watermill.LogFields
}

// NewLogger creates a Logger instance.
func NewLogger(logger *log.Logger, level LogLevel) Logger {
	return &loggerext{
		logger: logger,
		level:  level,
	}
}

func (l *loggerext) Error(msg string, err error, fields watermill.LogFields) {
	if l.level.Enabled(ERROR) {
		if err == nil {
			l.log(ERROR, msg, fields)
		} else {
			l.log(ERROR, msg, fields.Add(watermill.LogFields{"err": err}))
		}
	}
}

func (l *loggerext) Errorf(format string, a ...interface{}) {
	if l.level.Enabled(ERROR) {
		l.log(ERROR, fmt.Sprintf(format, a...), nil)
	}
}

func (l *loggerext) Warn(msg string, err error, fields watermill.LogFields) {
	if l.level.Enabled(WARN) {
		if err == nil {
			l.log(WARN, msg, fields)
		} else {
			l.log(WARN, msg, fields.Add(watermill.LogFields{"err": err}))
		}
	}
}

func (l *loggerext) Warnf(format string, a ...interface{}) {
	if l.level.Enabled(WARN) {
		l.log(WARN, fmt.Sprintf(format, a...), nil)
	}
}

func (l *loggerext) Info(msg string, fields watermill.LogFields) {
	if l.level.Enabled(INFO) {
		l.log(INFO, msg, fields)
	}
}

func (l *loggerext) Infof(format string, a ...interface{}) {
	if l.level.Enabled(INFO) {
		l.log(INFO, fmt.Sprintf(format, a...), nil)
	}
}

func (l *loggerext) Debug(msg string, fields watermill.LogFields) {
	if l.IsDebugEnabled() {
		l.log(DEBUG, msg, fields)
	}
}

func (l *loggerext) Debugf(format string, a ...interface{}) {
	if l.IsDebugEnabled() {
		l.log(DEBUG, fmt.Sprintf(format, a...), nil)
	}
}

func (l *loggerext) Trace(msg string, fields watermill.LogFields) {
	if l.IsTraceEnabled() {
		l.log(TRACE, msg, fields)
	}
}

func (l *loggerext) Tracef(format string, a ...interface{}) {
	if l.IsTraceEnabled() {
		l.log(TRACE, fmt.Sprintf(format, a...), nil)
	}
}

func (l *loggerext) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &loggerext{
		logger: l.logger,
		level:  l.level,
		fields: l.fields.Add(fields),
	}
}

func (l *loggerext) IsDebugEnabled() bool {
	return l.level.Enabled(DEBUG)
}

func (l *loggerext) IsTraceEnabled() bool {
	return l.level.Enabled(TRACE)
}

func (l *loggerext) log(logLevel LogLevel, message string, fields watermill.LogFields) {
	if l.fields != nil {
		if fields == nil {
			fields = l.fields
		} else {
			fields = l.fields.Add(fields)
		}
	}

	if len(fields) == 0 {
		l.logger.Println(logLevel.StringAligned(), message)
	} else {
		keys := make([]string, 0, len(fields))
		for field := range fields {
			keys = append(keys, field)
		}
		sort.Strings(keys)

		sb := loggerPool.Get().(*strings.Builder)
		sb.Reset()
		defer loggerPool.Put(sb)

		sb.WriteString(message)
		sb.WriteRune(' ')

		for _, key := range keys {
			writePair(sb, key, fields[key])
			sb.WriteRune(' ')
		}

		l.logger.Println(logLevel.StringAligned(), sb.String())
	}
}

func writePair(sb *strings.Builder, key string, value interface{}) {
	var valueStr string

	if stringer, ok := value.(fmt.Stringer); ok {
		valueStr = stringer.String()
	} else {
		valueStr = fmt.Sprintf("%v", value)
	}

	sb.WriteString(key)
	sb.WriteRune('=')

	if strings.ContainsRune(valueStr, ' ') {
		sb.WriteRune('"')
		sb.WriteString(valueStr)
		sb.WriteRune('"')
	} else {
		sb.WriteString(valueStr)
	}
}
