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

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package connector_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/tevino/abool/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/tests"

	"go.uber.org/goleak"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

const (
	payloadID     = "payload"
	headersId     = "headers"
	correlationID = "correlation-id"
)

type testMarshaler struct{}

func newTestMarshaller() conn.Marshaller {
	return &testMarshaler{}
}

func (testMarshaler) Marshal(msg *message.Message) ([]byte, error) {
	env := make(map[string]interface{})

	env[correlationID] = msg.UUID
	env[payloadID] = base64.StdEncoding.EncodeToString(msg.Payload)

	headers := map[string]string(msg.Metadata)
	if len(headers) > 0 {
		env[headersId] = headers
	}

	return json.Marshal(env)
}

func (testMarshaler) Unmarshal(mid uint16, payload []byte) (*message.Message, error) {
	env := make(map[string]interface{})

	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}

	body, err := base64.StdEncoding.DecodeString(env[payloadID].(string))
	if err != nil {
		return nil, err
	}

	m := message.NewMessage(env[correlationID].(string), body)

	if h, ok := env[headersId]; ok {
		headers := h.(map[string]interface{})
		for k, v := range headers {
			m.Metadata.Set(k, fmt.Sprintf("%v", v))
		}
	}

	return m, nil
}

type errorMarshaler struct{}

func newErrorMarshaler() conn.Marshaller {
	return &errorMarshaler{}
}

func (errorMarshaler) Marshal(msg *message.Message) ([]byte, error) {
	return nil, errors.New("marshal")
}

func (errorMarshaler) Unmarshal(mid uint16, payload []byte) (*message.Message, error) {
	return nil, errors.New("unmarshal")
}

type closedecorator struct {
	conn *conn.MQTTConnection

	pub message.Publisher

	closed *abool.AtomicBool
}

func newCloseDecorator(conn *conn.MQTTConnection, pub message.Publisher) message.Publisher {
	return &closedecorator{
		conn:   conn,
		pub:    pub,
		closed: abool.New(),
	}
}

func (p *closedecorator) Publish(topic string, messages ...*message.Message) error {
	if p.closed.IsSet() {
		return conn.ErrClosed
	}

	return p.pub.Publish(topic, messages...)
}

func (p *closedecorator) Close() error {
	if p.closed.SetToIf(false, true) {
		defer p.conn.Disconnect()

		return p.pub.Close()
	}
	return nil
}

func newPub(cfg *conn.Configuration, logger logger.Logger, codec conn.Marshaller) (message.Publisher, error) {
	pubClient, err := conn.NewMQTTConnection(cfg, "", logger)
	if err != nil {
		return nil, err
	}

	future := pubClient.Connect()
	<-future.Done()
	if err := future.Error(); err != nil {
		return nil, err
	}

	publisher := conn.NewPublisher(pubClient, conn.QosAtLeastOnce, logger, codec)

	return newCloseDecorator(pubClient, publisher), nil
}

func createPubSub(t *testing.T) (message.Publisher, message.Subscriber) {
	logger := testutil.NewLogger("connector", logger.INFO)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	publisher, err := newPub(config, logger, newTestMarshaller())
	require.NoError(t, err)

	var clientIdMap sync.Map
	subscriber := conn.NewDynamicSubscriber(
		config,
		conn.QosAtLeastOnce, true, logger,
		newTestMarshaller(),
		func(topic string) string {
			clientId, _ := clientIdMap.LoadOrStore(topic, watermill.NewShortUUID())
			return clientId.(string)
		},
	)

	return publisher, subscriber
}

func createPubSubDurable(t *testing.T, consumerGroup string) (message.Publisher, message.Subscriber) {
	logger := testutil.NewLogger("connector", logger.INFO)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	publisher, err := newPub(config, logger, newTestMarshaller())
	require.NoError(t, err)

	subscriber := conn.NewDynamicSubscriber(
		config,
		conn.QosAtLeastOnce, false, logger,
		newTestMarshaller(),
		func(topic string) string {
			return consumerGroup
		},
	)

	return publisher, subscriber
}

func TestPersistent(t *testing.T) {
	defer goleak.VerifyNone(t)

	features := tests.Features{
		Persistent:                          true,
		ExactlyOnceDelivery:                 false,
		RequireSingleInstance:               true,
		GuaranteedOrder:                     true,
		GuaranteedOrderWithSingleSubscriber: true,
		ConsumerGroups:                      true,
	}
	testFuncs := []struct {
		Func        func(t *testing.T, tCtx tests.TestContext, pubSubConstructor tests.PubSubConstructor)
		FuncName    string
		NotParallel bool
	}{
		{
			Func:     tests.TestPublishSubscribe,
			FuncName: "TestPublishSubscribe",
		},
		{
			Func:     tests.TestConcurrentSubscribe,
			FuncName: "TestConcurrentSubscribe",
		},
		{
			Func:     tests.TestNoAck,
			FuncName: "TestNoAck",
		},
		{
			Func:     tests.TestContinueAfterSubscribeClose,
			FuncName: "TestContinueAfterSubscribeClose",
		},
		{
			Func:     tests.TestPublishSubscribeInOrder,
			FuncName: "TestPublishSubscribeInOrder",
		},
		{
			Func:     tests.TestPublisherClose,
			FuncName: "TestPublisherClose",
		},
		{
			Func:     tests.TestTopic,
			FuncName: "TestTopic",
		},
		{
			Func:     tests.TestMessageCtx,
			FuncName: "TestMessageCtx",
		},
		{
			Func:     tests.TestPublishSubscribeInOrder,
			FuncName: "TestPublishSubscribeInOrder",
		},
	}

	for i := range testFuncs {
		testFunc := testFuncs[i]

		t.Run(testFunc.FuncName, func(t *testing.T) {
			if !testFunc.NotParallel {
				t.Parallel()
			}
			testID := tests.NewTestID()

			t.Run(string(testID), func(t *testing.T) {
				tCtx := tests.TestContext{
					TestID:   testID,
					Features: features,
				}

				testFunc.Func(t, tCtx, createPubSub)
			})
		})
	}

	t.Run("TestConsumerGroups", func(t *testing.T) {
		testID := tests.NewTestID()
		t.Run(string(testID), func(t *testing.T) {
			tCtx := tests.TestContext{
				TestID:   testID,
				Features: features,
			}

			tests.TestConsumerGroups(t, tCtx, createPubSubDurable)
		})
	})
}

func TestNonPersistent(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	config.CleanSession = true
	require.NoError(t, err)

	pubClient, err := conn.NewMQTTConnection(
		config, "",
		testutil.NewLogger("connector", logger.INFO),
	)
	require.NoError(t, err)

	future := pubClient.Connect()
	<-future.Done()
	require.NoError(t, future.Error())

	defer func() {
		pubClient.Disconnect()
		time.Sleep(time.Second)
	}()

	sub := conn.NewBufferedSubscriber(
		pubClient, 16, false,
		testutil.NewLogger("connector", logger.INFO),
		newTestMarshaller(),
	)

	pub := conn.NewPublisher(
		pubClient,
		conn.QosAtMostOnce,
		testutil.NewLogger("connector", logger.INFO),
		newTestMarshaller(),
	)

	doSimpleTest(t, pub, sub)
}

func doSimpleTest(t *testing.T, pub message.Publisher, sub message.Subscriber) {
	messagesCount := 1000

	if testing.Short() {
		messagesCount = 100
	}

	topicName := "test_topic_" + watermill.NewUUID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	msgs, err := sub.Subscribe(ctx, topicName)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	sendMessages := tests.PublishSimpleMessages(t, messagesCount, pub, topicName)
	receivedMsgs, _ := subscriber.BulkRead(msgs, messagesCount, time.Second)

	tests.AssertAllMessagesReceived(t, sendMessages, receivedMsgs)

	assert.NoError(t, pub.Close())
	assert.NoError(t, sub.Close())
}

func TestForwarder(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("connector", logger.DEBUG)

	config, err := testutil.NewLocalConfig()
	config.NoOpStore = true
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(
		config, watermill.NewShortUUID(),
		logger,
	)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())

	defer client.Disconnect()

	sub := conn.NewSubscriber(client, conn.QosAtLeastOnce, false, logger, nil)
	defer sub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := sub.Subscribe(ctx, "simple/test/#")
	require.NoError(t, err)

	pub := conn.NewPublisher(client, conn.QosAtMostOnce, logger, nil)
	msg := message.NewMessage("hello", []byte("Hello world"))
	msg.SetContext(conn.SetTopicToCtx(msg.Context(), "simple/test/demo"))
	msg.SetContext(conn.SetQosToCtx(msg.Context(), conn.QosAtLeastOnce))
	require.NoError(t, pub.Publish(conn.TopicEmpty, msg))

	select {
	case <-time.After(10 * time.Second):
		require.Fail(t, "Message not delivered")

	case msg, ok := <-messages:
		if ok {
			logger.Info("Message received: "+string(msg.Payload), nil)
			msg.Ack()
		} else {
			require.Fail(t, "Message not delivered")
		}
	}
}

func TestClosedPubSub(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	pubClient, err := conn.NewMQTTConnection(
		config, "",
		testutil.NewLogger("connector", logger.INFO),
	)
	require.NoError(t, err)

	pub := conn.NewPublisher(
		pubClient,
		conn.QosAtMostOnce,
		testutil.NewLogger("connector", logger.INFO),
		newTestMarshaller(),
	)

	require.NoError(t, pub.Close())
	require.Error(t, pub.Publish("simple/test", message.NewMessage("test", []byte("test"))))

	sub := conn.NewSubscriber(pubClient, conn.QosAtMostOnce, false, nil, nil)
	require.NoError(t, sub.Close())

	_, err = sub.Subscribe(context.Background(), "simple/test")
	require.ErrorIs(t, err, conn.ErrClosed)

	dynSub := conn.NewDynamicSubscriber(
		config,
		conn.QosAtLeastOnce, false, nil,
		nil,
		func(topic string) string {
			return ""
		},
	)

	if subscribeInitializer, ok := dynSub.(message.SubscribeInitializer); ok {
		require.Error(t, subscribeInitializer.SubscribeInitialize("simple/#/test"))
	}

	_, err = dynSub.Subscribe(context.Background(), "simple/test")
	require.NoError(t, err)
	require.NoError(t, dynSub.Close())

	if subscribeInitializer, ok := dynSub.(message.SubscribeInitializer); ok {
		require.ErrorIs(t, subscribeInitializer.SubscribeInitialize("simple/test"), conn.ErrClosed)
	}

	_, err = dynSub.Subscribe(context.Background(), "simple/test")
	require.ErrorIs(t, err, conn.ErrClosed)
}

func TestSubscriptionManager(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("connector", logger.INFO)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	sub := conn.NewSubscriber(client, conn.QosAtMostOnce, false, logger, nil)
	defer sub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := sub.Subscribe(ctx, conn.TopicEmpty)
	require.NoError(t, err)

	manager := conn.NewSubscriptionManager()
	manager.Add("remove/test/#")
	manager.ForwardTo(client)

	manager.Add("simple/test/#")
	manager.Remove("remove/test/#")

	done := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-time.After(10 * time.Second):
				done <- false
				return

			case msg, ok := <-messages:
				if !ok {
					done <- false
					return
				}

				if topic, ok := conn.TopicFromCtx(msg.Context()); ok {
					if strings.HasPrefix(topic, "simple/test/") {
						logger.Info("Message received: "+string(msg.Payload), nil)
						msg.Ack()
						done <- true
						return
					}
				}
				msg.Nack()
			}
		}
	}()

	pub := conn.NewPublisher(client, conn.QosAtMostOnce, logger, nil)

	wrong := message.NewMessage("wrong", []byte("This is never delivered"))
	require.NoError(t, pub.Publish("removed/test/demo", wrong))

	msg := message.NewMessage("hello", []byte("Hello world"))
	require.NoError(t, pub.Publish("simple/test/demo", msg))

	if ok := <-done; !ok {
		require.Fail(t, "Message not delivered")
	}
}

func TestOnlinePublisher(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	sub := conn.NewSubscriber(client, conn.QosAtLeastOnce, true, nil, nil)
	defer sub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, err := sub.Subscribe(ctx, "sync/publish/test/#")
	require.NoError(t, err)

	pub := conn.NewOnlinePublisher(client, conn.QosAtLeastOnce, 10*time.Second, nil, nil)
	msg := message.NewMessage("hello", []byte("test"))
	require.NoError(t, pub.Publish("sync/publish/test/demo", msg))

	msg, ok := <-messages
	require.True(t, ok)
	defer msg.Ack()

	assert.Equal(t, "test", string(msg.Payload))
}

func TestOnlinePublisherClosed(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	pub := conn.NewOnlinePublisher(client, conn.QosAtMostOnce, 10*time.Second, nil, nil)
	require.NoError(t, pub.Close())

	msg := message.NewMessage("hello", []byte("test"))
	assert.ErrorIs(t, conn.ErrClosed, pub.Publish("errors/test", msg))
}

func TestDoNothingNack(t *testing.T) {
	defer goleak.VerifyNone(t)

	messagesCount := 3
	topicName := watermill.NewUUID()

	pub, sub := createPubSub(t)
	defer func() {
		err := pub.Close()
		require.NoError(t, err)

		err = sub.Close()
		require.NoError(t, err)
	}()

	if subscribeInitializer, ok := sub.(message.SubscribeInitializer); ok {
		require.NoError(t, subscribeInitializer.SubscribeInitialize(topicName))
	}

	tests.PublishSimpleMessages(t, messagesCount, pub, topicName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages, err := sub.Subscribe(ctx, topicName)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	for i := 0; i < messagesCount; i++ {
		select {
		case msg, closed := <-messages:
			if !closed {
				t.Fatal("messages channel closed unexpetedly")
			}
			msg.Nack()

		case <-time.After(10 * time.Second):
			require.Fail(t, "Message not delivered")
		}
	}
}

func TestSimpleDelivery(t *testing.T) {
	defer goleak.VerifyNone(t)

	//Create logger
	logger := testutil.NewLogger("connector", logger.DEBUG)

	//Create configuration
	config, err := testutil.NewLocalConfig()
	config.ConnectRetryInterval = 10 * time.Second
	require.NoError(t, err)

	//Create connection
	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	timeout, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	//Connect to mosquitto
	future := client.Connect()
	select {
	case <-future.Done():
		require.NoError(t, future.Error())
		defer client.Disconnect()

	case <-timeout.Done():
		client.Disconnect()
		require.Fail(t, "Failed to connect to local broker")
	}

	//Create subscribtion
	sub := conn.NewSubscriber(client, conn.QosAtLeastOnce, true, logger, nil)
	messages, err := sub.Subscribe(context.Background(), "simple/test/#")
	require.NoError(t, err)
	defer sub.Close()

	//Send message
	pub := conn.NewPublisher(client, conn.QosAtMostOnce, logger, nil)
	msg := message.NewMessage("hello", []byte("Hello world"))
	require.NoError(t, pub.Publish("simple/test/demo", msg))

	if msg, ok := subscriber.BulkRead(messages, 1, time.Second*10); ok {
		logFields := watermill.LogFields{
			"url":      client.URL(),
			"payload":  string(msg[0].Payload),
			"client_d": client.ClientID(),
		}
		logger.Info("Message published", logFields)
	} else {
		require.Fail(t, "Message not delivered")
	}
}

type connectionEventListener struct {
	events chan<- bool
}

func (h *connectionEventListener) Connected(connected bool, err error) {
	h.events <- connected
}

func TestConnectionEvents(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	events := make(chan bool, 8)
	l1 := &connectionEventListener{
		events: events,
	}
	client.AddConnectionListener(l1)

	l2 := &connectionEventListener{
		events: events,
	}
	client.AddConnectionListener(l2)
	client.AddConnectionListener(l2)
	client.RemoveConnectionListener(l2)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	client.Disconnect()

	//Check for connected event
	assert.True(t, <-events)
	//Check for disconnected event
	assert.False(t, <-events)
}

func TestMarshalError(t *testing.T) {
	defer goleak.VerifyNone(t)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	config.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		ClientAuth:         tls.NoClientCert,
	}

	client, err := conn.NewMQTTConnection(config, watermill.NewShortUUID(), nil)
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	sub := conn.NewSubscriber(client, conn.QosAtLeastOnce, true, nil, newErrorMarshaler())
	messages, err := sub.Subscribe(context.Background(), "marshall/error,unmarshall/error")
	require.NoError(t, err)
	defer sub.Close()

	errPub := conn.NewPublisher(client, conn.QosAtMostOnce, nil, newErrorMarshaler())
	assert.Error(t, errPub.Publish("marshall/error", message.NewMessage("err1", []byte("{}"))))

	pub := conn.NewPublisher(client, conn.QosAtLeastOnce, nil, nil)
	require.NoError(t, pub.Publish("unmarshall/error", message.NewMessage("err2", []byte("{}"))))

	if _, ok := subscriber.BulkRead(messages, 1, time.Second*3); ok {
		require.Fail(t, "Message delivered")
	}
}

func TestSubscriberBlocking(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("connector", logger.INFO)

	topicName := watermill.NewUUID()
	clientId := watermill.NewShortUUID()

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	config.MaxReconnectInterval = 0
	config.KeepAliveTimeout = 4 * time.Second

	sub := conn.NewDynamicSubscriber(
		config,
		conn.QosAtLeastOnce, true,
		logger, nil,
		func(topic string) string {
			return clientId
		},
	)

	if subscribeInitializer, ok := sub.(message.SubscribeInitializer); ok {
		require.NoError(t, subscribeInitializer.SubscribeInitialize(topicName))
	}

	r, err := message.NewRouter(
		message.RouterConfig{
			CloseTimeout: 15 * time.Second,
		},
		logger,
	)
	require.NoError(t, err)

	messages := make(chan bool, 2)

	r.AddNoPublisherHandler(
		"test_blocking_handler",
		topicName,
		sub,
		func(msg *message.Message) error {
			if logger.IsDebugEnabled() {
				logFields := watermill.LogFields{
					"at": time.Now().Format(time.RFC3339Nano),
				}
				logger.Debug("Received message", logFields)
			}

			time.Sleep(6 * time.Second)
			messages <- true
			return nil
		},
	)

	done := make(chan bool, 1)
	go func() {
		defer func() {
			done <- true
		}()

		if err := r.Run(context.Background()); err != nil {
			logger.Error("Failed to start router", err, nil)
		}
	}()

	defer func() {
		r.Close()

		<-done
	}()

	<-r.Running()

	pubConfig, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	pub, err := newPub(pubConfig, logger, nil)
	require.NoError(t, err)
	defer pub.Close()

	msg := message.NewMessage(watermill.NewUUID(), []byte("blocking"))
	assert.NoError(t, pub.Publish(topicName, msg))

	select {
	case <-time.After(10 * time.Second):
		require.Fail(t, "Message not delivered")

	case _, ok := <-messages:
		if !ok {
			require.Fail(t, "Message not delivered")
		}
	}
}

func TestUnroutableMessages(t *testing.T) {
	logger := watermill.NewCaptureLogger()

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	clientId := watermill.NewShortUUID()

	client, err := conn.NewMQTTConnection(config, clientId, logger)
	require.NoError(t, err)

	sub := conn.NewSubscriber(client, conn.QosAtLeastOnce, false, nil, nil)
	_, err = sub.Subscribe(context.Background(), "unroutable/#")
	require.NoError(t, err)

	_, err = sub.Subscribe(context.Background(), "test/#/unroutable")
	require.NoError(t, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	defer client.Disconnect()

	time.Sleep(5 * time.Second)

	require.NoError(t, sub.Close())

	pub := conn.NewPublisher(client, conn.QosAtLeastOnce, nil, nil)
	require.NoError(t, err)

	unroutable := message.NewMessage("unroutable", []byte("unroutable"))
	require.NoError(t, pub.Publish("unroutable/test", unroutable))

	time.Sleep(5 * time.Second)

	logFields := watermill.LogFields{
		"mqtt_url":  config.URL,
		"client_id": clientId,
		"topic":     "unroutable/test",
		"message":   "unroutable",
	}

	assert.True(t, logger.Has(watermill.CapturedMessage{
		Level:  watermill.ErrorLogLevel,
		Fields: logFields,
		Msg:    "Message not routed",
	}))
}

func TestCannotConnect(t *testing.T) {
	logger := testutil.NewLogger("connector", logger.ERROR)

	config, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	config.MaxReconnectInterval = 0
	config.ConnectRetryInterval = 0
	config.ConnectTimeout = 1 * time.Second

	u, err := url.Parse(config.URL)
	require.NoError(t, err)

	host, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)

	p, err := strconv.Atoi(port)
	require.NoError(t, err)

	url := url.URL{
		Scheme: u.Scheme,
		Host:   fmt.Sprintf("%s:%d", host, p+1),
	}
	config.URL = url.String()

	sub := conn.NewDynamicSubscriber(
		config,
		conn.QosAtLeastOnce, false, logger,
		nil,
		nil,
	)

	if subscribeInitializer, ok := sub.(message.SubscribeInitializer); ok {
		require.Error(t, subscribeInitializer.SubscribeInitialize("testing/#"))
	}

	_, err = sub.Subscribe(context.Background(), "testing/#")
	require.Error(t, err)
	require.NoError(t, sub.Close())
}

func TestCredentialsProvider(t *testing.T) {
	config, err := testutil.NewLocalConfig()

	username := config.Credentials.UserName
	pass := config.Credentials.Password

	config.Credentials.UserName = "invalid"
	config.Credentials.Password = "invalid"

	pubClient, err := conn.NewMQTTConnectionCredentialsProvider(
		config, "",
		nil,
		func() (string, string) {
			return username, pass
		},
	)
	require.NoError(t, err)

	future := pubClient.Connect()
	<-future.Done()
	require.NoError(t, future.Error())
	pubClient.Disconnect()
}
