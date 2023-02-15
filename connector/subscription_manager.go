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
	topics map[string]int
	lock   sync.Mutex

	conn atomic.Value
}

func (m *subscriptionManager) Add(topic string) bool {
	if m.add0(topic) {
		if connRef := m.conn.Load(); connRef != nil {
			conn := connRef.(*MQTTConnection)
			conn.subscribe(nil, QosAtMostOnce, topic)
			return true
		}
	}
	return false
}

func (m *subscriptionManager) add0(topic string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if c, ok := m.topics[topic]; ok {
		m.topics[topic] = c + 1
		return false
	}

	m.topics[topic] = 1
	return true
}

func (m *subscriptionManager) Remove(topic string) bool {
	if m.remove0(topic) {
		if connRef := m.conn.Load(); connRef != nil {
			conn := connRef.(*MQTTConnection)
			conn.unsubscribe(topic, true)
			return true
		}
	}
	return false
}

func (m *subscriptionManager) remove0(topic string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if c, ok := m.topics[topic]; ok {
		if c > 1 {
			m.topics[topic] = c - 1
			return false
		}
		delete(m.topics, topic)
		return true
	}

	return false
}

func (m *subscriptionManager) ForwardTo(conn *MQTTConnection) {
	m.conn.Store(conn)

	topics := m.copyTopics()

	for _, topic := range topics {
		conn.subscribe(nil, QosAtMostOnce, topic)
	}
}

func (m *subscriptionManager) copyTopics() []string {
	m.lock.Lock()
	defer m.lock.Unlock()

	result := make([]string, 0)
	for topic := range m.topics {
		result = append(result, topic)
	}
	return result
}

// NewSubscriptionManager creates subscriptions manager.
func NewSubscriptionManager() SubscriptionManager {
	return &subscriptionManager{
		topics: make(map[string]int),
	}
}
