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
	"time"

	"github.com/pkg/errors"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

type publisher struct {
	*wrapper

	conn *MQTTConnection

	qos Qos

	ackTimeout time.Duration

	fields watermill.LogFields
}

// NewPublisher creates a publisher for a MQTT connection.
func NewPublisher(conn *MQTTConnection,
	qos Qos,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Publisher {
	return newPublisher(conn, qos, 0, "Publisher", logger, marshaller)
}

// NewSyncPublisher creates a synchronized publisher for a MQTT connection.
func NewSyncPublisher(conn *MQTTConnection,
	qos Qos,
	ackTimeout time.Duration,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Publisher {
	return newPublisher(conn, qos, ackTimeout, "SyncPublisher", logger, marshaller)
}

// Publish publishes the messages with the provided topic.
func (m *publisher) Publish(topic string, msgs ...*message.Message) error {
	if m.isClosed() {
		return ErrClosed
	}

	m.processWg.Add(1)
	defer m.processWg.Done()

	for _, msg := range msgs {
		payload, err := m.marshaller.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "marshal failed")
		}

		publishTopic := topic
		if msgTopic, ok := TopicFromCtx(msg.Context()); ok {
			if len(msgTopic) > 0 {
				publishTopic = msgTopic
			}
		}

		publishQos := m.qos
		if qos, ok := QosFromCtx(msg.Context()); ok {
			publishQos = qos
		}

		retain := RetainFromCtx(msg.Context())

		token := m.conn.mqttClient.Publish(publishTopic, byte(publishQos), retain, payload)

		if m.ackTimeout > 0 && publishQos != QosAtMostOnce {
			if !token.WaitTimeout(m.ackTimeout) {
				return ErrTimeout
			}

			if err := token.Error(); err != nil {
				return err
			}
		}

		if isDebugEnabled(m.logger) {
			logFields := m.fields.Add(
				watermill.LogFields{
					"message_uuid": msg.UUID,
					"clientid":     m.conn.clientID,
					"mqtt_topic":   publishTopic,
					"qos":          publishQos,
					"retain":       retain,
				})
			m.logger.Debug("Message published", logFields)
		}
	}

	return nil
}

func newPublisher(conn *MQTTConnection,
	qos Qos,
	ackTimeout time.Duration,
	name string,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Publisher {
	wrapper := newWrapper(&conn.config, logger, marshaller)
	wrapper.name = name

	pub := &publisher{
		wrapper:    wrapper,
		conn:       conn,
		qos:        qos,
		ackTimeout: ackTimeout,
		fields: watermill.LogFields{
			"mqtt_url": wrapper.cfg.URL,
		},
	}

	return pub
}

type onlinepub struct {
	conn *MQTTConnection

	pub message.Publisher
}

// NewOnlinePublisher creates publisher for the provided connection in online mode.
func NewOnlinePublisher(conn *MQTTConnection,
	qos Qos,
	ackTimeout time.Duration,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Publisher {

	return &onlinepub{
		conn: conn,
		pub:  NewSyncPublisher(conn, qos, ackTimeout, logger, marshaller),
	}
}

// Publish publishes the messages with the provided topic.
func (p *onlinepub) Publish(topic string, messages ...*message.Message) error {
	if !p.conn.isConnected() {
		return ErrNotConnected
	}

	return p.pub.Publish(topic, messages...)
}

// Close closes the publisher.
func (p *onlinepub) Close() error {
	return p.pub.Close()
}
