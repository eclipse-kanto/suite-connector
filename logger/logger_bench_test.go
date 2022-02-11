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
	"io"
	"log"
	"math/rand"
	"testing"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/eclipse-kanto/suite-connector/logger"
)

func BenchmarkLoggerNoFields(b *testing.B) {
	l1 := logger.NewLogger(log.New(io.Discard, "[bench] ", log.Lmsgprefix), logger.TRACE)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l1.Trace("Testing logger performance", nil)
	}
}

func BenchmarkLoggerWithFormatting(b *testing.B) {
	l1 := logger.NewLogger(log.New(io.Discard, "[bench] ", log.Lmsgprefix), logger.TRACE)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l1.Tracef("Formatting benchmark a=%d b=%d c=%d ", 1, 2, 3)
	}
}

func BenchmarkLoggerWithFields(b *testing.B) {
	fields := watermill.LogFields{
		"c": 3,
		"b": 2,
		"a": 1,
	}

	l1 := logger.NewLogger(log.New(io.Discard, "[bench] ", log.Lmsgprefix), logger.TRACE)
	l1 = l1.With(fields).(logger.Logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l1.Info("Testing logger performance",
			watermill.LogFields{
				"d": rand.Intn(1000),
			},
		)
	}
}

func BenchmarkLoggerDecorator(b *testing.B) {
	d := logger.NewLevelDecorator(log.New(io.Discard, "[bench] ", log.Lmsgprefix), logger.TRACE)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Printf("Formatting benchmark a=%d b=%d c=%d ", 1, 2, 3)
	}
}
