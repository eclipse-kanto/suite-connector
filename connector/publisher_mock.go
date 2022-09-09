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

package connector

import (
	"github.com/tevino/abool/v2"

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

	closed *abool.AtomicBool
}

// Publish handles publishing the messages per topic.
func (p *errpublisher) Publish(topic string, msgs ...*message.Message) error {
	if p.closed.IsSet() {
		return ErrClosed
	}
	return p.err
}

func (p *errpublisher) Close() error {
	if p.closed.SetToIf(false, true) {
		return nil
	}
	return ErrClosed
}

// NewErrPublisher creates publisher that fails on every publish with specified error
func NewErrPublisher(err error) message.Publisher {
	return &errpublisher{
		err:    err,
		closed: abool.New(),
	}
}
