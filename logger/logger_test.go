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

package logger_test

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/eclipse-kanto/suite-connector/logger"
)

func TestLogging(t *testing.T) {
	doTestLogging(t, logger.TRACE)
	doTestLogging(t, logger.DEBUG)
	doTestLogging(t, logger.INFO)
	doTestLogging(t, logger.WARN)
	doTestLogging(t, logger.ERROR)
}

func doTestLogging(t *testing.T, lvl logger.LogLevel) {
	fields := watermill.LogFields{
		"c": 3,
		"b": 2,
		"a": 1,
	}

	b1 := new(bytes.Buffer)
	l1 := logger.NewLogger(log.New(b1, "[log] ", log.Lmsgprefix), lvl)
	l1.Error("Simple message", nil, fields)
	l1.Warn("Simple message", nil, fields)
	l1.Info("Simple message", fields)
	l1.Debug("Simple message", fields)
	l1.Trace("Simple message", fields)

	b2 := new(bytes.Buffer)
	l2 := logger.NewLogger(log.New(b2, "[log] ", log.Lmsgprefix), lvl)
	l2.Errorf("Simple message a=%d b=%d c=%d ", 1, 2, 3)
	l2.Warnf("Simple message a=%d b=%d c=%d ", 1, 2, 3)
	l2.Infof("Simple message a=%d b=%d c=%d ", 1, 2, 3)
	l2.Debugf("Simple message a=%d b=%d c=%d ", 1, 2, 3)
	l2.Tracef("Simple message a=%d b=%d c=%d ", 1, 2, 3)

	assert.Equal(t, b1.Bytes(), b2.Bytes())
}

type Point struct {
	X, Y int
}

func (p Point) String() string {
	return fmt.Sprintf("(%d,%d)", p.X, p.Y)
}

func TestLogInheritance(t *testing.T) {
	fields := watermill.LogFields{
		"c": 3,
		"b": 2,
		"a": 1,
		"p": Point{
			X: 1,
			Y: 2,
		},
	}

	b1 := new(bytes.Buffer)

	var l1 logger.Logger
	l1 = logger.NewLogger(log.New(b1, "[log] ", log.Lmsgprefix), logger.INFO)
	l1 = l1.With(fields).(logger.Logger)
	l1.Error("Simple message", nil, nil)
	l1.Error("Simple message", errors.New("simple test"), nil)
	l1.Warn("Simple warning", nil, nil)
	l1.Warn("Simple warning", errors.New("simple warning"), nil)

	p := Point{X: 1, Y: 2}
	b2 := new(bytes.Buffer)
	l2 := logger.NewLogger(log.New(b2, "[log] ", log.Lmsgprefix), logger.INFO)
	l2.Errorf("Simple message a=%d b=%d c=%d p=%v ", 1, 2, 3, p)
	l2.Errorf("Simple message a=%d b=%d c=%d err=%q p=%v ", 1, 2, 3, "simple test", p)
	l2.Warnf("Simple warning a=%d b=%d c=%d p=%v ", 1, 2, 3, p)
	l2.Warnf("Simple warning a=%d b=%d c=%d err=%q p=%v ", 1, 2, 3, "simple warning", p)

	assert.Equal(t, b1.Bytes(), b2.Bytes())
}
