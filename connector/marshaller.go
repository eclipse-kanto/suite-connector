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
	"strconv"

	"github.com/ThreeDotsLabs/watermill/message"
)

// Marshaller interface is used to enable marshall and unmarshal functionality for messages.
type Marshaller interface {
	Marshal(*message.Message) ([]byte, error)
	Unmarshal(mid uint16, payload []byte) (*message.Message, error)
}

// NewDefaultMarshaller returns the default marshaller.
func NewDefaultMarshaller() Marshaller {
	return &defMarshaller{}
}

type defMarshaller struct{}

func (*defMarshaller) Marshal(msg *message.Message) ([]byte, error) {
	return msg.Payload, nil
}

func (*defMarshaller) Unmarshal(mid uint16, payload []byte) (*message.Message, error) {
	return message.NewMessage(strconv.FormatUint(uint64(mid), 10), payload), nil
}
