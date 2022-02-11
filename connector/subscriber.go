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
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const cannotSubscribeForTopics = "cannot subscribe for topics: %v"

// ClientIDProvider defines the subscriber function per topic.
type ClientIDProvider func(topic string) string

type subscriber struct {
	*wrapper

	conn *MQTTConnection

	qos      Qos
	buffSize int

	autoDelete bool
}

// NewSubscriber creates a message subscriber.
func NewSubscriber(conn *MQTTConnection,
	qos Qos,
	autoDelete bool,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Subscriber {
	wrapper := newWrapper(&conn.config, logger, marshaller)
	wrapper.name = "Subscriber"

	return &subscriber{
		wrapper:    wrapper,
		conn:       conn,
		qos:        qos,
		autoDelete: autoDelete,
	}
}

// NewBufferedSubscriber creates a buffered message subscriber.
func NewBufferedSubscriber(conn *MQTTConnection,
	buffSize int,
	autoDelete bool,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
) message.Subscriber {
	wrapper := newWrapper(&conn.config, logger, marshaller)
	wrapper.name = "BufferedSubscriber"

	return &subscriber{
		wrapper:    wrapper,
		conn:       conn,
		buffSize:   buffSize,
		qos:        QosAtMostOnce,
		autoDelete: autoDelete,
	}
}

// Subscribe subscribes per topic using the proved context.
func (m *subscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	if m.isClosed() {
		return nil, ErrClosed
	}

	topics := parseSubs(topic)

	logFields := watermill.LogFields{
		"mqtt_url": m.conn.config.URL,
		"topics":   topics,
	}

	loop := &messageloop{
		out:        make(chan *message.Message, m.buffSize),
		logFields:  logFields,
		logger:     m.logger,
		closing:    m.closing,
		ctx:        ctx,
		marshaller: m.marshaller,
		autoAck:    m.buffSize > 0,
		timeout:    calcTimeout(m.cfg.KeepAliveTimeout),
	}

	m.processWg.Add(1)

	if len(topic) == 0 {
		m.conn.setDefaultHandler(loop.handler)

		loop.runMessageLoop(func() {
			m.conn.setDefaultHandler(nil)
		}, func() {
			m.processWg.Done()
		})
	} else {
		subID := m.conn.subscribe(loop.handler, m.qos, topics...)

		loop.runMessageLoop(func() {
			m.conn.unsubscribe(subID, m.autoDelete)
		}, func() {
			m.processWg.Done()
		})
	}

	return loop.out, nil
}

type dynamicsubscriber struct {
	*wrapper

	qos        Qos
	autoDelete bool

	clientIDProvider ClientIDProvider
}

// NewDynamicSubscriber creates a dynamic message subscriber.
func NewDynamicSubscriber(cfg *Configuration,
	qos Qos,
	autoDelete bool,
	logger watermill.LoggerAdapter,
	marshaller Marshaller,
	clientIDProvider ClientIDProvider,
) message.Subscriber {
	config := *cfg
	wrapper := newWrapper(&config, logger, marshaller)
	wrapper.name = "NewDynamicSubscriber"

	sub := &dynamicsubscriber{
		wrapper:          wrapper,
		qos:              qos,
		autoDelete:       autoDelete,
		clientIDProvider: clientIDProvider,
	}

	return sub
}

// SubscribeInitialize initializes the subscription 1for the provided topic.
func (m *dynamicsubscriber) SubscribeInitialize(topic string) error {
	if m.isClosed() {
		return ErrClosed
	}

	topics := parseSubs(topic)

	clientOpts := createClientOptions(m.cfg, m.getClientID(topic), false)

	logFields := watermill.LogFields{
		"mqtt_url": m.cfg.URL,
		"topics":   topics,
	}

	m.logger.Info("Initializing subscribe", logFields)

	mqttClient := mqtt.NewClient(clientOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	defer mqttClient.Disconnect(disconnectQuiesce)

	token := mqttClient.SubscribeMultiple(subFilters(m.qos, topics), nil)
	if !token.WaitTimeout(subTimeout) {
		return errors.Wrapf(ErrTimeout, cannotSubscribeForTopics, topic)
	}
	if err := token.Error(); err != nil {
		return errors.Wrapf(err, cannotSubscribeForTopics, topic)
	}

	return nil
}

func (m *dynamicsubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	if m.isClosed() {
		return nil, ErrClosed
	}

	topics := parseSubs(topic)

	clientOpts := createClientOptions(m.cfg, m.getClientID(topic), false)

	logFields := watermill.LogFields{
		"mqtt_url": m.cfg.URL,
		"topics":   topics,
	}

	loop := &messageloop{
		out:        make(chan *message.Message),
		logFields:  logFields,
		logger:     m.logger,
		closing:    m.closing,
		ctx:        ctx,
		marshaller: m.marshaller,
		timeout:    calcTimeout(m.cfg.KeepAliveTimeout),
	}

	m.processWg.Add(1)

	clientOpts.SetDefaultPublishHandler(loop.handler)

	mqttClient := mqtt.NewClient(clientOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		defer m.processWg.Done()

		loop.close()

		return nil, token.Error()
	}

	mqttClient.SubscribeMultiple(subFilters(m.qos, topics), nil)

	loop.runMessageLoop(func() {
		if m.autoDelete {
			logFields := watermill.LogFields{
				"mqtt_url": m.cfg.URL,
				"topics":   topics,
			}
			m.logger.Debug("Sending unsubscribe packet", logFields)

			token := mqttClient.Unsubscribe(topics...)
			if !token.WaitTimeout(unsubTimeout) {
				m.logger.Debug("Failed to unsubscribe to", logFields)
			}
		}
		mqttClient.Disconnect(disconnectQuiesce)
	}, func() {
		m.processWg.Done()
	})

	return loop.out, nil
}

func (m *dynamicsubscriber) getClientID(topic string) string {
	if m.clientIDProvider == nil {
		return watermill.NewShortUUID()
	}
	clientID := m.clientIDProvider(topic)
	if len(clientID) == 0 {
		clientID = watermill.NewShortUUID()
	}
	return clientID
}

type messageloop struct {
	out chan *message.Message

	marshaller Marshaller

	logFields watermill.LogFields

	logger  watermill.LoggerAdapter
	closing chan struct{}

	ctx context.Context

	autoAck bool
	timeout time.Duration

	topicClosed    int32
	processMessage sync.Mutex
}

func (l *messageloop) handler(c mqtt.Client, mqttMsg mqtt.Message) {
	l.processMessage.Lock()
	defer l.processMessage.Unlock()

	if atomic.LoadInt32(&l.topicClosed) == wrapperClosed {
		return
	}

	msg, err := l.marshaller.Unmarshal(mqttMsg.MessageID(), mqttMsg.Payload())
	if err != nil {
		markers := l.logFields.Add(
			watermill.LogFields{
				"uuid": mqttMsg.MessageID(),
			})
		l.logger.Error("Unmarshal failed", err, markers)
		return
	}

	markers := l.logFields.Add(watermill.LogFields{"uuid": msg.UUID})

	ctx := SetTopicToCtx(l.ctx, mqttMsg.Topic())
	ctx = SetQosToCtx(ctx, Qos(mqttMsg.Qos()))

	ctx, cancel := context.WithCancel(ctx)
	msg.SetContext(ctx)
	defer cancel()

	select {
	case l.out <- msg:
		l.logger.Trace("Message consumed", markers)

	case _, ok := <-l.closing:
		if !ok {
			l.logger.Info("Message not consumed, subscriber is closing", markers)
		}
		return
	}

	if l.autoAck {
		return
	}

	timer := time.NewTimer(l.timeout)
	select {
	case <-timer.C:
		l.logger.Info("Ack timeout", markers)

	case _, ok := <-l.closing:
		timer.Stop()
		if !ok {
			l.logger.Info("Message not consumed, subscriber is closing", markers)
		}
		return

	case <-msg.Acked():
		timer.Stop()
		l.logger.Trace("Message acked", markers)

	case <-msg.Nacked():
		timer.Stop()
		l.logger.Trace("Message nacked", markers)
	}
}

func (l *messageloop) runMessageLoop(stopper, cleaner func()) {
	go func() {
		defer cleaner()
		defer l.close()

		select {
		case _, ok := <-l.closing:
			if !ok {
				l.logger.Trace("Subscriber close was invoked", l.logFields)
			}

		case <-l.ctx.Done():
			l.logger.Trace("Subscriber context was cancelled", l.logFields)
		}

		atomic.StoreInt32(&l.topicClosed, wrapperClosed)

		if stopper != nil {
			stopper()
		}

		l.processMessage.Lock()
		defer l.processMessage.Unlock()

		atomic.StoreInt32(&l.topicClosed, wrapperClosed)
	}()
}

func (l *messageloop) close() {
	close(l.out)
}
