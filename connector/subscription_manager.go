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
	"sync"
	"sync/atomic"
)

// SubscriptionManager is responsible for subscriptions management.
type SubscriptionManager interface {
	// Add adds a subscription for the provided topic. Returns true if it doesn't exist and was successfully subscribed.
	Add(topic string) bool

	// Remove removes a subscription for the provided topic. Returns true if it does exist and was successfully unsubscribed.
	Remove(topic string) bool

	// ForwardTo specifies the connection to which should be the messages forwarded.
	ForwardTo(conn *MQTTConnection)
}

type subscriptionManager struct {
	topics sync.Map

	conn atomic.Value
}

func (m *subscriptionManager) Add(topic string) bool {
	if _, ok := m.topics.LoadOrStore(topic, true); !ok {
		if connRef := m.conn.Load(); connRef != nil {
			conn := connRef.(*MQTTConnection)
			conn.subscribe(nil, QosAtMostOnce, topic)
			return true
		}
	}
	return false
}

func (m *subscriptionManager) Remove(topic string) bool {
	if _, ok := m.topics.LoadAndDelete(topic); ok {
		if connRef := m.conn.Load(); connRef != nil {
			conn := connRef.(*MQTTConnection)
			conn.unsubscribe(topic, true)
			return true
		}
	}
	return false
}

func (m *subscriptionManager) ForwardTo(conn *MQTTConnection) {
	m.conn.Store(conn)

	m.topics.Range(func(key, value interface{}) bool {
		topic := key.(string)
		conn.subscribe(nil, QosAtMostOnce, topic)
		return true
	})
}

// NewSubscriptionManager creates subscriptions manager.
func NewSubscriptionManager() SubscriptionManager {
	return new(subscriptionManager)
}
