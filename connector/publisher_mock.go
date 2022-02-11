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
	"sync/atomic"

	"github.com/ThreeDotsLabs/watermill/message"
)

type nullpublisher struct{}

func (nullpublisher) Publish(topic string, messages ...*message.Message) error {
	return nil
}

func (nullpublisher) Close() error {
	return nil
}

// NullPublisher creates publisher that discards all published messages
func NullPublisher() message.Publisher {
	return new(nullpublisher)
}

type errpublisher struct {
	err error

	closed int32
}

// Publish handles publishing the messages per topic.
func (p *errpublisher) Publish(topic string, msgs ...*message.Message) error {
	if atomic.LoadInt32(&p.closed) == wrapperClosed {
		return ErrClosed
	}
	return p.err
}

func (p *errpublisher) Close() error {
	if atomic.CompareAndSwapInt32(&p.closed, wrapperOpened, wrapperClosed) {
		return nil
	}
	return ErrClosed
}

// NewErrPublisher creates publisher that fails on every publish with specified error
func NewErrPublisher(err error) message.Publisher {
	return &errpublisher{
		err: err,
	}
}
