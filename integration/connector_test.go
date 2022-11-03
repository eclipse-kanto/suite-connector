// Copyright (c) 2022 Contributors to the Eclipse Foundation
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

//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/eclipse/ditto-clients-golang"
	"github.com/eclipse/ditto-clients-golang/model"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/eclipse/ditto-clients-golang/protocol/things"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/websocket"
)

type testConfig struct {
	Broker                   string `def:"tcp://localhost:1883"`
	MqttQuiesceMs            int    `def:"500"`
	MqttAcknowledgeTimeoutMs int    `def:"3000"`

	DigitalTwinAPIAddress string

	DigitalTwinAPIUser     string `def:"ditto"`
	DigitalTwinAPIPassword string `def:"ditto"`

	EventTimeoutMs  int `def:"30000"`
	StatusTimeoutMs int `def:"10000"`

	TimeDeltaMs int `def:"5000"`
}

type thingConfig struct {
	DeviceID string `json:"deviceId"`
	TenantID string `json:"tenantId"`
	PolicyID string `json:"policyId"`
}

type ConnectorSuite struct {
	suite.Suite

	mqttClient  MQTT.Client
	dittoClient *ditto.Client

	cfg      *testConfig
	thingCfg *thingConfig

	thingURL   string
	featureURL string
}

const (
	featureID               = "ConnectorTestFeature"
	propertyName            = "testProperty"
	commandName             = "testCommand"
	responsePayloadTemplate = "responsePayload: %s"
	https                   = "https"
	httpsDefaultPort        = "443"
	httpDefaultPort         = "80"
)

func (suite *ConnectorSuite) SetupSuite() {
	cfg := &testConfig{}

	suite.T().Log(getConfigHelp(*cfg))

	if err := initConfigFromEnv(cfg); err != nil {
		suite.T().Fatal(err)
	}

	suite.T().Logf("test config: %+v", *cfg)

	opts := MQTT.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(uuid.New().String()).
		SetKeepAlive(30 * time.Second).
		SetCleanSession(true).
		SetAutoReconnect(true)

	mqttClient := MQTT.NewClient(opts)

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		require.NoError(suite.T(), token.Error(), "connect to MQTT broker")
	}

	thingCfg, err := getThingConfig(mqttClient)
	if err != nil {
		mqttClient.Disconnect(uint(cfg.MqttQuiesceMs))
		require.NoError(suite.T(), err, "get thing config")
	}

	suite.T().Logf("thing config: %+v", *thingCfg)

	dittoClient, err := ditto.NewClientMQTT(mqttClient, ditto.NewConfiguration())
	if err == nil {
		err = dittoClient.Connect()
	}

	if err != nil {
		mqttClient.Disconnect(uint(cfg.MqttQuiesceMs))
		require.NoError(suite.T(), err, "initialize ditto client")
	}

	feature := &model.Feature{}
	feature.WithProperty(propertyName, "testValue")

	cmd := things.NewCommand(model.NewNamespacedIDFrom(thingCfg.DeviceID)).Twin().Feature(featureID).
		Modify(feature)
	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = dittoClient.Send(msg)
	require.NoError(suite.T(), err, "create test feature")

	dittoClient.Subscribe(func(requestID string, msg *protocol.Envelope) {
		if msg.Path == fmt.Sprintf("/features/%s/inbox/messages/%s", featureID, commandName) {
			value, ok := msg.Value.(string)
			if !ok {
				suite.T().Fatalf("unexpected message payload: %v, %T\n", msg.Value, msg.Value)
			}
			responsePayload := fmt.Sprintf(responsePayloadTemplate, value)
			response := things.NewMessage(model.NewNamespacedID(msg.Topic.Namespace, msg.Topic.EntityName))
			// respond to the message by using the outbox
			path := strings.Replace(msg.Path, "/inbox/", "/outbox/", 1)
			responseMsg := response.Envelope(
				protocol.WithCorrelationID(msg.Headers.CorrelationID()),
				protocol.WithResponseRequired(false),
				protocol.WithContentType("application/json")).
				WithTopic(msg.Topic).
				WithPath(path).
				WithValue(responsePayload).
				WithStatus(http.StatusOK)
			if err := dittoClient.Reply(requestID, responseMsg); err != nil {
				suite.T().Fatalf("failed to send response: %v\n", err)
			}
		}
	})

	suite.dittoClient = dittoClient
	suite.mqttClient = mqttClient
	suite.cfg = cfg
	suite.thingCfg = thingCfg

	suite.thingURL = fmt.Sprintf("%s/api/2/things/%s", strings.TrimSuffix(cfg.DigitalTwinAPIAddress, "/"), thingCfg.DeviceID)
	suite.featureURL = fmt.Sprintf("%s/features/%s", suite.thingURL, featureID)
}

func (suite *ConnectorSuite) TearDownSuite() {
	if _, err := suite.doRequest(http.MethodDelete, suite.featureURL); err != nil {
		suite.T().Logf("error while deleting test feature: %v", err)
	}

	suite.dittoClient.Disconnect()
	suite.mqttClient.Disconnect(uint(suite.cfg.MqttQuiesceMs))
}

func TestConnectorSuite(t *testing.T) {
	suite.Run(t, new(ConnectorSuite))
}

func (suite *ConnectorSuite) TestConnectionStatus() {
	type connectionStatus struct {
		ReadySince time.Time `json:"readySince"`
		ReadyUntil time.Time `json:"readyUntil"`
	}

	timeout := time.Duration(suite.cfg.StatusTimeoutMs * int(time.Millisecond))
	threshold := time.Now().Add(timeout)

	for {
		statusURL := fmt.Sprintf("%s/features/ConnectionStatus/properties/status", suite.thingURL)
		body, err := suite.doRequest("GET", statusURL)
		if err != nil {
			if time.Now().Before(threshold) {
				continue
			}
			suite.T().Errorf("connection status property not available: %v", err)
			break
		}

		status := &connectionStatus{}
		err = json.Unmarshal(body, status)
		require.NoError(suite.T(), err, "connection status should be parsed")

		suite.T().Logf("%+v", status)
		suite.T().Logf("current time: %v", time.Now())

		forever := time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
		if status.ReadyUntil != forever {
			if time.Now().Before(threshold) {
				continue
			}
			suite.T().Errorf("readyUntil should be %v", forever)
			break
		}

		delta := int64(suite.cfg.TimeDeltaMs)
		assert.Less(suite.T(), status.ReadySince.UnixMilli(), time.Now().UnixMilli()+delta, "readySince should be before current time")
		break
	}
}

func (suite *ConnectorSuite) TestCommand() {
	ws, err := suite.newWSConnection()
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(namespace)
	setCommandTopic(suite, cmd, namespace)
	cmd.Path = fmt.Sprintf("/features/%s/inbox/messages/%s", featureID, commandName)
	cmd.Payload = "request"
	correlationID := uuid.New().String()
	cmdMsg := cmd.Envelope(
		protocol.WithCorrelationID(correlationID),
		protocol.WithContentType("text/plain"))

	respCh := suite.beginWSWait(ws, func(payload []byte) bool {
		respMsg := &protocol.Envelope{}
		if err := json.Unmarshal(payload, respMsg); err != nil {
			suite.T().Error(err)
			return true
		}

		expectedPath := strings.ReplaceAll(cmd.Path, "inbox", "outbox")
		assert.Equal(suite.T(), expectedPath, respMsg.Path, "expected response path")
		assert.Equal(suite.T(), *cmd.Topic, *respMsg.Topic, "expected topic")
		assert.Equal(suite.T(), respMsg.Status, http.StatusOK, "expected http ok")
		assert.Equal(suite.T(), fmt.Sprintf(responsePayloadTemplate, cmd.Payload), respMsg.Value, "expected response payload")

		return true
	})

	err = websocket.JSON.Send(ws, cmdMsg)
	require.NoError(suite.T(), err, "unable to send command to the backend via websocket")

	require.True(suite.T(), <-respCh, "command response should be received")
}

func setCommandTopic(suite *ConnectorSuite, cmd *things.Command, namespace *model.NamespacedID) {
	topic := &protocol.Topic{}
	topicStr := fmt.Sprintf("%s/%s/things/live/messages/%s", namespace.Namespace, namespace.Name, commandName)
	err := topic.UnmarshalJSON([]byte("\"" + topicStr + "\""))
	require.NoError(suite.T(), err, "unable to create topic for command test")
	cmd.Topic = topic
}

func (suite *ConnectorSuite) TestEvent() {
	suite.testModify("e", "testEvent")
}

func (suite *ConnectorSuite) TestTelemetry() {
	suite.testModify("t", "testTelemetry")
}

func (suite *ConnectorSuite) testModify(channel string, newValue string) {
	ws, err := suite.newWSConnection()
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	const subAck = "START-SEND-EVENTS:ACK"
	ackCh := suite.beginWSWait(ws, func(payload []byte) bool {
		ack := strings.TrimSpace(string(payload))
		return ack == subAck
	})

	sub := fmt.Sprintf("START-SEND-EVENTS?filter=like(resource:path,'/features/%s/*')", featureID)
	err = websocket.Message.Send(ws, sub)
	require.NoError(suite.T(), err, "unable to listen for events by using a websocket connection")

	ok := <-ackCh
	require.True(suite.T(), ok, "acknowledgement %v should be received", subAck)

	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(namespace).Twin().
		FeatureProperty(featureID, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	eventCh := suite.beginWSWait(ws, func(payload []byte) bool {
		props := make(map[string]interface{})

		err := json.Unmarshal(payload, &props)
		if err == nil {
			suite.T().Logf("event received: %v", props)

			return props["topic"] == fmt.Sprintf("%s/%s/things/twin/events/modified", namespace.Namespace, namespace.Name) &&
				props["path"] == fmt.Sprintf("/features/%s/properties/%s", featureID, propertyName) &&
				props["value"] == newValue

		}

		suite.T().Logf("error while waiting for event: %v", err)
		return false
	})

	err = suite.sendDittoEvent(channel, msg)

	require.NoError(suite.T(), err, "unable to send event to the backend")

	require.True(suite.T(), <-eventCh, "property changed event should be received")

	propertyURL := fmt.Sprintf("%s/properties/%s", suite.featureURL, propertyName)
	body, err := suite.doRequest("GET", propertyURL)
	require.NoError(suite.T(), err, "unable to get feature property")

	assert.Equal(suite.T(), fmt.Sprintf("\"%s\"", newValue), strings.TrimSpace(string(body)), "property value updated")
}

func (suite *ConnectorSuite) sendDittoEvent(topic string, message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	token := suite.mqttClient.Publish(topic, 1, false, payload)
	timeout := time.Duration(suite.cfg.MqttAcknowledgeTimeoutMs * int(time.Millisecond))
	if !token.WaitTimeout(timeout) {
		return ditto.ErrAcknowledgeTimeout
	}
	return token.Error()
}

func (suite *ConnectorSuite) beginWSWait(ws *websocket.Conn, check func(payload []byte) bool) chan bool {
	timeout := time.Duration(suite.cfg.EventTimeoutMs * int(time.Millisecond))

	ch := make(chan bool)

	go func() {
		resultCh := make(chan bool)

		go func() {
			var payload []byte
			threshold := time.Now().Add(timeout)
			for time.Now().Before(threshold) {
				err := websocket.Message.Receive(ws, &payload)
				if err == nil && check(payload) {
					resultCh <- true
					return
				}

				suite.T().Logf("error while waiting for WS message: %v", err)
			}

			suite.T().Logf("WS response not received in %v", timeout)

			resultCh <- false
		}()

		select {
		case result := <-resultCh:
			ch <- result
		case <-time.After(timeout):
			ws.Close()
			ch <- false
		}
	}()

	return ch
}

func (suite *ConnectorSuite) newWSConnection() (*websocket.Conn, error) {
	wsAddress, err := asWSAddress(suite.cfg.DigitalTwinAPIAddress)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ws/2", wsAddress)
	cfg, err := websocket.NewConfig(url, suite.cfg.DigitalTwinAPIAddress)
	if err != nil {
		return nil, err
	}

	auth := fmt.Sprintf("%s:%s", suite.cfg.DigitalTwinAPIUser, suite.cfg.DigitalTwinAPIPassword)
	enc := base64.StdEncoding.EncodeToString([]byte(auth))
	cfg.Header = http.Header{
		"Authorization": {"Basic " + enc},
	}

	return websocket.DialConfig(cfg)
}

func getPortOrDefault(url *url.URL, defaultPort string) string {
	port := url.Port()
	if port == "" {
		return defaultPort
	}
	return port
}

func asWSAddress(address string) (string, error) {
	url, err := url.Parse(address)
	if err != nil {
		return "", err
	}

	if url.Scheme == https {
		return fmt.Sprintf("wss://%s:%s", url.Hostname(), getPortOrDefault(url, httpsDefaultPort)), nil
	}

	return fmt.Sprintf("ws://%s:%s", url.Hostname(), getPortOrDefault(url, httpDefaultPort)), nil
}

func (suite *ConnectorSuite) doRequest(method string, url string) ([]byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(suite.cfg.DigitalTwinAPIUser, suite.cfg.DigitalTwinAPIPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s request failed: %s", method, url, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func getThingConfig(mqttClient MQTT.Client) (*thingConfig, error) {
	type result struct {
		cfg *thingConfig
		err error
	}

	ch := make(chan result)

	if token := mqttClient.Subscribe("edge/thing/response", 1, func(client MQTT.Client, message MQTT.Message) {
		var cfg thingConfig
		if err := json.Unmarshal(message.Payload(), &cfg); err != nil {
			ch <- result{nil, err}
		}
		ch <- result{&cfg, nil}
	}); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	if token := mqttClient.Publish("edge/thing/request", 1, false, ""); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	timeout := 5 * time.Second
	select {
	case result := <-ch:
		return result.cfg, result.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("thing config not received in %v", timeout)
	}
}
