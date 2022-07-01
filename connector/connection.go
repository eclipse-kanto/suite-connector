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
	"fmt"
	"sync"
	"time"

	"github.com/tevino/abool/v2"

	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"

	"github.com/ThreeDotsLabs/watermill"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

// Qos defines the quality of service.
type Qos byte

const (
	// QosAtMostOnce defines at most once value.
	QosAtMostOnce Qos = iota
	// QosAtLeastOnce defines at least once value.
	QosAtLeastOnce
)

const (
	// TopicEmpty defines empty topic.
	TopicEmpty = ""

	externalRetrySleep = 5 * time.Second
)

// ConnectFuture defines connection behavior.
type ConnectFuture interface {
	Done() <-chan struct{}

	Error() error
}

// ConnectionListener is used to notify on connection state changes.
type ConnectionListener interface {
	Connected(connected bool, err error)
}

// NewMQTTConnection creates a local MQTT connection.
func NewMQTTConnection(
	config *Configuration, clientID string, logger watermill.LoggerAdapter,
) (*MQTTConnection, error) {
	return NewMQTTConnectionCredentialsProvider(config, clientID, logger, nil)
}

// NewMQTTConnectionCredentialsProvider creates a local MQTT connection with specific credentials provider.
func NewMQTTConnectionCredentialsProvider(
	config *Configuration, clientID string, logger watermill.LoggerAdapter, prov func() (string, string),
) (*MQTTConnection, error) {
	if config == nil {
		return nil, errors.New("no client config")
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	if logger == nil {
		logger = watermill.NopLogger{}
	}

	cfg := *config

	if len(clientID) == 0 {
		cfg.CleanSession = true
		clientID = watermill.NewShortUUID()
	}

	conn := &MQTTConnection{
		config:       cfg,
		logger:       logger,
		clientID:     clientID,
		reconnecting: abool.New(),
		running:      abool.NewBool(true),
	}

	conn.initMQTTClient(prov)

	return conn, nil
}

func createClientOptions(config *Configuration, clientID string, cleanSession bool) *mqtt.ClientOptions {
	clientOpts := mqtt.NewClientOptions().
		SetClientID(clientID).
		SetCleanSession(cleanSession).
		AddBroker(config.URL).
		SetProtocolVersion(4).
		SetOrderMatters(true).
		SetKeepAlive(config.KeepAliveTimeout).
		SetConnectTimeout(config.ConnectTimeout).
		SetMaxReconnectInterval(config.MaxReconnectInterval)

	if config.MaxReconnectInterval > 0 {
		clientOpts.SetAutoReconnect(!config.ExternalReconnect)
	} else {
		clientOpts.SetAutoReconnect(false)
	}

	if config.ConnectRetryInterval > 0 {
		clientOpts.SetConnectRetry(true)
		clientOpts.SetConnectRetryInterval(config.ConnectRetryInterval)
	}

	if config.TLSConfig != nil {
		clientOpts.SetTLSConfig(config.TLSConfig)
	}

	if len(config.WillMessage) > 0 {
		clientOpts.SetBinaryWill(
			config.WillTopic,
			config.WillMessage,
			byte(config.WillQos),
			config.WillRetain,
		)
	}

	if len(config.Credentials.UserName) > 0 {
		clientOpts.SetCredentialsProvider(func() (username string, password string) {
			return config.Credentials.UserName, config.Credentials.Password
		})
	}

	if config.NoOpStore {
		store := new(nostore)
		clientOpts.SetStore(store)
	} else {
		clientOpts.SetStore(mqtt.NewOrderedMemoryStore())
	}

	return clientOpts
}

type subscriptioninfo struct {
	topics []string

	callback mqtt.MessageHandler

	qos Qos

	disposed bool
	lock     sync.Mutex
}

// MQTTConnection holds a MQTT connection data and manages the communication.
type MQTTConnection struct {
	config Configuration

	logger watermill.LoggerAdapter

	clientID string

	mqttClient mqtt.Client

	topics sync.Map

	listenersLock sync.Mutex
	listeners     []ConnectionListener

	defHandlerLock sync.Mutex
	defaultHandler mqtt.MessageHandler

	running      *abool.AtomicBool
	reconnecting *abool.AtomicBool
	stopGroup    sync.WaitGroup
}

func (c *MQTTConnection) isConnected() bool {
	return c.mqttClient.IsConnectionOpen()
}

// Connect opens the connection.
func (c *MQTTConnection) Connect() ConnectFuture {
	c.running.Set()

	return c.mqttClient.Connect().(*mqtt.ConnectToken)
}

// ConnectBackoff creates the connection backoff.
func (c *MQTTConnection) ConnectBackoff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()

	b.MaxElapsedTime = 0
	b.RandomizationFactor = 0.25
	b.Multiplier = c.config.BackoffMultiplier

	if c.config.MinReconnectInterval > 0 {
		b.InitialInterval = c.config.MinReconnectInterval
	}

	if c.config.MaxReconnectInterval > 0 {
		b.MaxInterval = c.config.MaxReconnectInterval
	}

	return b
}

// URL returns the connection address.
func (c *MQTTConnection) URL() string {
	return c.config.URL
}

// ClientID returns the connection client ID.
func (c *MQTTConnection) ClientID() string {
	return c.clientID
}

// Disconnect closes the connection.
func (c *MQTTConnection) Disconnect() {
	fireDisconnect := c.mqttClient.IsConnected()

	if c.running.SetToIf(true, false) {
		c.stopGroup.Wait()

		c.mqttClient.Disconnect(disconnectQuiesce)
	}

	if fireDisconnect {
		c.fireConnectionEvent(false, nil)

		c.logger.Info("Connection was closed", watermill.LogFields{
			"mqtt_url":  c.config.URL,
			"client_id": c.clientID,
		})
	}
}

// AddConnectionListener adds a connection listener.
func (c *MQTTConnection) AddConnectionListener(listener ConnectionListener) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	for i := range c.listeners {
		if listener == c.listeners[i] {
			return
		}
	}

	var newListeners []ConnectionListener
	if c.listeners != nil {
		clen := len(c.listeners)
		newListeners = make([]ConnectionListener, 1+clen)
		copy(newListeners, c.listeners)
		newListeners[clen] = listener
	} else {
		newListeners = append(newListeners, listener)
	}

	c.listeners = newListeners
}

// RemoveConnectionListener removes a connection listener.
func (c *MQTTConnection) RemoveConnectionListener(listener ConnectionListener) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	for i := range c.listeners {
		if listener == c.listeners[i] {
			newListeners := make([]ConnectionListener, len(c.listeners)-1)
			copy(newListeners, c.listeners[:i])
			copy(newListeners[i:], c.listeners[i+1:])

			c.listeners = newListeners
			break
		}
	}
}

func (c *MQTTConnection) initMQTTClient(prov func() (string, string)) {
	config := &c.config

	clientOpts := createClientOptions(config, c.clientID, config.CleanSession)
	clientOpts.SetOnConnectHandler(c.onConnected)
	clientOpts.SetConnectionLostHandler(c.onConnectionLost)
	clientOpts.SetDefaultPublishHandler(c.onDefaultHandlerWrapper)

	if prov != nil {
		clientOpts.SetCredentialsProvider(prov)
	}

	c.mqttClient = mqtt.NewClient(clientOpts)
}

func (c *MQTTConnection) listenersRef() []ConnectionListener {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	return c.listeners
}

func (c *MQTTConnection) fireConnectionEvent(hasConnection bool, err error) {
	listenersRef := c.listenersRef()

	for _, l := range listenersRef {
		l.Connected(hasConnection, err)
	}
}

func (c *MQTTConnection) defHandlerRef() mqtt.MessageHandler {
	c.defHandlerLock.Lock()
	defer c.defHandlerLock.Unlock()

	return c.defaultHandler
}

func (c *MQTTConnection) setDefaultHandler(defaultHandler mqtt.MessageHandler) {
	c.defHandlerLock.Lock()
	defer c.defHandlerLock.Unlock()

	c.defaultHandler = defaultHandler
}

func (c *MQTTConnection) onConnected(client mqtt.Client) {
	logFields := watermill.LogFields{
		"client_id":     c.clientID,
		"mqtt_url":      c.config.URL,
		"clean_session": c.config.CleanSession,
	}
	c.logger.Info("Connected", logFields)

	c.stopGroup.Add(1)
	defer c.stopGroup.Done()

	var subTokens []*mqtt.SubscribeToken

	c.topics.Range(func(key, value interface{}) bool {
		sub := value.(*subscriptioninfo)

		sub.lock.Lock()
		defer sub.lock.Unlock()

		if sub.disposed {
			return true
		}

		subReq := subFilters(sub.qos, sub.topics)
		logFields["filters"] = subReq

		c.logger.Info("Sending subscribe packet", logFields)
		token := c.mqttClient.SubscribeMultiple(subReq, sub.callback)
		if subToken, ok := token.(*mqtt.SubscribeToken); ok {
			subTokens = append(subTokens, subToken)
		}
		return true
	})

	c.waitForSubscribeTokens(subTokens)

	c.fireConnectionEvent(true, nil)
}

func (c *MQTTConnection) waitForSubscribeTokens(tokens []*mqtt.SubscribeToken) {
	logFields := watermill.LogFields{
		"mqtt_url":  c.config.URL,
		"client_id": c.clientID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	for _, token := range tokens {
		logFields["filters"] = token.Result()

		select {
		case <-ctx.Done():
			c.logger.Info("Subscribe packet processed", logFields)

		case <-token.Done():
			if err := token.Error(); err != nil {
				c.logger.Error("Subscription problem", err, logFields)
			} else {
				c.logger.Info("Subscription done", logFields)
			}
		}
	}
}

func (c *MQTTConnection) onConnectionLost(client mqtt.Client, err error) {
	logFields := watermill.LogFields{
		"mqtt_url":  c.config.URL,
		"client_id": c.clientID,
	}

	c.logger.Error("Connection to mqtt lost", err, logFields)

	c.fireConnectionEvent(false, err)

	if c.config.ExternalReconnect {
		if c.reconnecting.SetToIf(false, true) {
			defer c.reconnecting.UnSet()

			c.externalReconnect(client)
		}
	}
}

func (c *MQTTConnection) onDefaultHandlerWrapper(client mqtt.Client, msg mqtt.Message) {
	handlerRef := c.defHandlerRef()
	if handlerRef != nil {
		handlerRef(client, msg)
	} else {
		logFields := watermill.LogFields{
			"mqtt_url":  c.config.URL,
			"client_id": c.clientID,
			"topic":     msg.Topic(),
			"message":   string(msg.Payload()),
		}

		c.logger.Error("Message not routed", nil, logFields)
	}
}

func (c *MQTTConnection) subscribe(callback mqtt.MessageHandler, qos Qos, topics ...string) string {
	id := topics[0]
	if len(topics) > 1 {
		id = watermill.NewUUID()
	}

	c.topics.Store(id, &subscriptioninfo{
		topics:   topics,
		callback: callback,
		qos:      qos,
	})

	if c.mqttClient.IsConnectionOpen() {
		logFields := watermill.LogFields{
			"mqtt_url":  c.config.URL,
			"client_id": c.clientID,
			"qos":       qos,
			"topics":    topics,
		}
		c.logger.Info("Sending subscribe packet", logFields)
		c.mqttClient.SubscribeMultiple(subFilters(qos, topics), callback)
	} else {
		if qos != QosAtMostOnce {
			for _, topic := range topics {
				c.mqttClient.AddRoute(topic, callback)
			}
		}
	}

	return id
}

func (c *MQTTConnection) unsubscribe(id string, autoDelete bool) {
	value, loaded := c.topics.LoadAndDelete(id)

	if !loaded {
		return
	}

	sub := value.(*subscriptioninfo)

	sub.lock.Lock()
	defer sub.lock.Unlock()

	sub.disposed = true

	if autoDelete && c.mqttClient.IsConnectionOpen() {
		logFields := watermill.LogFields{
			"mqtt_url":  c.config.URL,
			"client_id": c.clientID,
			"topics":    sub.topics,
		}
		c.logger.Info("Sending unsubscribe packet", logFields)
		c.mqttClient.Unsubscribe(sub.topics...)
	} else {
		for _, topic := range sub.topics {
			c.mqttClient.AddRoute(topic, c.onDefaultHandlerWrapper)
		}
	}
}

func (c *MQTTConnection) externalReconnect(client mqtt.Client) {
	logFields := watermill.LogFields{
		"mqtt_url":  c.config.URL,
		"client_id": c.clientID,
	}

	c.stopGroup.Add(1)
	defer c.stopGroup.Done()

	b := c.ConnectBackoff()
	b.Reset()

	for {
		if c.running.IsNotSet() {
			return
		}

		reconnectInterval := b.NextBackOff()
		if reconnectInterval == backoff.Stop {
			c.logger.Error("Reconnect stopped", nil, logFields)
			return
		}

		spinCount := int(reconnectInterval / (externalRetrySleep))
		if spinCount == 0 {
			spinCount = 1
		}

		c.logger.Debug(fmt.Sprintf("Reconnect after %v", reconnectInterval.Round(time.Second)), logFields)

		for i := 0; i < spinCount; i++ {
			time.Sleep(externalRetrySleep)

			if c.running.IsNotSet() {
				return
			}
		}

		future := client.Connect()
		<-future.Done()

		if err := future.Error(); err != nil {
			c.logger.Error("Reconnect failed", err, logFields)

			//Handle forced connection close by the broker
			if errors.Is(err, packets.ErrorRefusedBadUsernameOrPassword) {
				c.fireConnectionEvent(false, err)
				return
			}

			if errors.Is(err, packets.ErrorRefusedNotAuthorised) {
				c.fireConnectionEvent(false, err)
				return
			}

		} else {
			return
		}
	}
}
