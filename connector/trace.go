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
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"unsafe"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// NewTrace creates the tracing as middleware handler.
func NewTrace(logger watermill.LoggerAdapter, prefixes []string) message.HandlerMiddleware {
	if logger == nil {
		panic("Nil logger")
	}

	var tracerPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	logMessage := func(msg *message.Message) error {
		buff := tracerPool.Get().(*bytes.Buffer)
		defer tracerPool.Put(buff)

		buff.Reset()

		if err := json.Compact(buff, msg.Payload); err != nil {
			return err
		}

		logFields := watermill.LogFields{
			"message_uuid": msg.UUID,
		}

		b := buff.Bytes()
		mdata := *(*string)(unsafe.Pointer(&b))
		logger.Debug(mdata, logFields)

		return nil
	}

	return func(h message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			if len(msg.Payload) == 0 {
				return h(msg)
			}

			if msgTopic, ok := TopicFromCtx(msg.Context()); ok {
				for _, pref := range prefixes {
					if strings.HasPrefix(msgTopic, pref) {
						if err := logMessage(msg); err != nil {
							return nil, err
						}
						break
					}
				}
			}

			return h(msg)
		}
	}
}
