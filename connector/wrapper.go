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

package connector

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/pkg/errors"
)

const (
	wrapperOpened = iota
	wrapperClosed
)

var (
	// ErrClosed defines a closed publisher or subscriber channel error.
	ErrClosed = errors.New("publisher/subscriber closed")
	// ErrTimeout defines a publish timeout error.
	ErrTimeout = errors.New("publish timeout")
	// ErrNotConnected defines error on not connected state.
	ErrNotConnected = errors.New("not connected")
)

type wrapper struct {
	name string
	cfg  *Configuration

	logger watermill.LoggerAdapter

	marshaller Marshaller

	closed  int32
	closing chan struct{}

	processWg sync.WaitGroup
}

func newWrapper(cfg *Configuration, logger watermill.LoggerAdapter, marshaller Marshaller) *wrapper {
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	if marshaller == nil {
		marshaller = NewDefaultMarshaller()
	}

	return &wrapper{
		cfg:        cfg,
		logger:     logger,
		marshaller: marshaller,
		closed:     wrapperOpened,
		closing:    make(chan struct{}),
	}
}

func (m *wrapper) doClose() bool {
	return !atomic.CompareAndSwapInt32(&m.closed, wrapperOpened, wrapperClosed)
}

func (m *wrapper) isClosed() bool {
	return atomic.LoadInt32(&m.closed) == wrapperClosed
}

// Close closes the messaging channel.
func (m *wrapper) Close() error {
	if m.doClose() {
		return nil
	}

	logFields := watermill.LogFields{
		"mqtt_url": m.cfg.URL,
	}

	close(m.closing)

	m.processWg.Wait()

	m.logger.Debug(fmt.Sprintf("%s message processing stopped", m.name), logFields)

	return nil
}
