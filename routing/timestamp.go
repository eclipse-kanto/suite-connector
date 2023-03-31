// Copyright (c) 2023 Contributors to the Eclipse Foundation
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

package routing

import (
	"time"

	"github.com/tidwall/sjson"

	"github.com/ThreeDotsLabs/watermill/message"
)

// AddTimestamp creates middleware handler which adds timestamp to message body
func AddTimestamp(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		timestamp := time.Now().UnixMilli()
		if payload, err := sjson.SetBytes(msg.Payload, "headers.x-timestamp", timestamp); err != nil {
			return nil, err
		} else {
			event := message.NewMessage(msg.UUID, payload)
			event.SetContext(msg.Context())

			return h(event)
		}
	}
}
