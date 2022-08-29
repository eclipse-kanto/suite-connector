// Copyright (c) 2022 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0
//
// SPDX-License-Identifier: EPL-2.0

package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/eclipse/ditto-clients-golang"
	"github.com/eclipse/ditto-clients-golang/model"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/eclipse/ditto-clients-golang/protocol/things"
	mqtt "github.com/eclipse/paho.mqtt.golang"
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

	DittoHost string
	DittoPort int

	DittoUser     string `def:"ditto"`
	DittoPassword string `def:"ditto"`

	EventTimeoutMs int `def:"30000"`
}

type thingConfig struct {
	DeviceID string `json:"deviceId"`
	TenantID string `json:"tenantId"`
	PolicyID string `json:"policyId"`
}

type commandPayload struct {
	Topic   string                 `json:"topic"`
	Headers map[string]interface{} `json:"headers"`
	Path    string                 `json:"path"`
	Value   string                 `json:"value"`
}

type commandResponsePayload struct {
	commandPayload
	Status int `json:"status"`
}

type ConnectorSuite struct {
	suite.Suite

	mqttClient  mqtt.Client
	dittoClient *ditto.Client

	cfg      *testConfig
	thingCfg *thingConfig

	thingURL   string
	featureURL string
}

const (
	envVariablesPrefix    = "SCT"
	featureName           = "ConnectorTestFeature"
	propertyName          = "testProperty"
	commandName           = "testCommand"
	commandResponseFormat = "response[%s]"
)

func (suite *ConnectorSuite) SetupSuite() {
	cfg := &testConfig{}

	suite.T().Log(getConfigHelp(*cfg, envVariablesPrefix))

	if err := initConfigFromEnv(cfg, envVariablesPrefix); err != nil {
		suite.T().Skip(err)
	}

	suite.T().Logf("test config: %+v", *cfg)

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(uuid.New().String()).
		SetKeepAlive(30 * time.Second).
		SetCleanSession(true).
		SetAutoReconnect(true)

	mqttClient := mqtt.NewClient(opts)

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

	cmd := things.NewCommand(model.NewNamespacedIDFrom(thingCfg.DeviceID)).Twin().Feature(featureName).
		Modify(feature)
	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = dittoClient.Send(msg)
	require.NoError(suite.T(), err, "create test feature")

	dittoClient.Subscribe(func(requestID string, msg *protocol.Envelope) {
		if msg.Path != fmt.Sprintf("/features/%s/inbox/messages/%s", featureName, commandName) {
			suite.T().Logf("unexpected command: %s", msg.Path)
			return
		}

		headers := protocol.NewHeaders(protocol.WithCorrelationID(msg.Headers.CorrelationID()),
			protocol.WithContentType("application/json"))

		value, ok := msg.Value.(string)
		if !ok {
			suite.T().Errorf("unexpected message payload: %v, %T", msg.Value, msg.Value)
			return
		}

		response := fmt.Sprintf(commandResponseFormat, value)

		reply := &protocol.Envelope{
			Topic:   msg.Topic,
			Headers: headers,
			Path:    strings.Replace(msg.Path, "/inbox/", "/outbox/", 1),
			Value:   response,
			Status:  http.StatusNoContent,
		}

		if err := dittoClient.Reply(requestID, reply); err != nil {
			suite.T().Error("failed to send response:", err)
		}
	})

	suite.dittoClient = dittoClient
	suite.mqttClient = mqttClient
	suite.cfg = cfg
	suite.thingCfg = thingCfg

	suite.thingURL = fmt.Sprintf("http://%s:%d/api/2/things/%s", cfg.DittoHost, cfg.DittoPort, thingCfg.DeviceID)
	suite.featureURL = fmt.Sprintf("%s/features/%s", suite.thingURL, featureName)
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
	statusURL := fmt.Sprintf("%s/features/ConnectionStatus/properties/status", suite.thingURL)
	body, err := suite.doRequest("GET", statusURL)
	require.NoError(suite.T(), err, "connection status property should be available")

	type connectionStatus struct {
		ReadySince time.Time `json:"readySince"`
		ReadyUntil time.Time `json:"readyUntil"`
	}

	status := &connectionStatus{}
	err = json.Unmarshal(body, status)
	require.NoError(suite.T(), err, "connection status should be parsed")

	require.True(suite.T(), time.Now().After(status.ReadySince), "readySince should be BEFORE current time")
	require.True(suite.T(), time.Now().Before(status.ReadyUntil), "readyUntil should be AFTER current time")
}

func (suite *ConnectorSuite) TestCommand() {
	ws, err := suite.newWSConnection()
	defer ws.Close()

	cmd := commandPayload{}
	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd.Topic = fmt.Sprintf("%s/%s/things/live/messages/%s", namespace.Namespace, namespace.Name, commandName)
	cmd.Path = fmt.Sprintf("/features/%s/inbox/messages/%s", featureName, commandName)
	cmd.Value = "request"

	correlationID := uuid.New().String()
	cmd.Headers = map[string]interface{}{"content-type": "text/plain", "correlation-id": correlationID}

	respCh := suite.beginWSWait(ws, func(payload []byte) bool {
		resp := &commandResponsePayload{}
		if err := json.Unmarshal(payload, resp); err != nil {
			suite.T().Error(err)
			return true
		}

		path := strings.ReplaceAll(cmd.Path, "inbox", "outbox")
		assert.Equal(suite.T(), path, resp.Path)
		assert.Equal(suite.T(), cmd.Topic, resp.Topic)
		assert.Equal(suite.T(), resp.Status, http.StatusNoContent)
		assert.Equal(suite.T(), fmt.Sprintf(commandResponseFormat, cmd.Value), resp.Value)

		return true
	})

	err = websocket.JSON.Send(ws, cmd)
	require.NoError(suite.T(), err)

	require.True(suite.T(), <-respCh, "command response should be received")
}

func (suite *ConnectorSuite) TestEvent() {
	suite.testModify("e", "testEvent")
}

func (suite *ConnectorSuite) TestTelemetry() {
	suite.testModify("t", "testTelemetry")
}

func (suite *ConnectorSuite) testModify(channel string, newValue string) {
	ws, err := suite.newWSConnection()
	defer ws.Close()

	const subAck = "START-SEND-EVENTS:ACK"
	ackCh := suite.beginWSWait(ws, func(payload []byte) bool {
		ack := strings.TrimSpace(string(payload))
		return ack == subAck
	})

	sub := fmt.Sprintf("START-SEND-EVENTS?filter=like(resource:path,'/features/%s/*')", featureName)
	err = websocket.Message.Send(ws, sub)
	require.NoError(suite.T(), err)

	ok := <-ackCh
	require.True(suite.T(), ok, "acknowledgement %v should be received", subAck)

	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(namespace).Twin().
		FeatureProperty(featureName, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	eventCh := suite.beginWSWait(ws, func(payload []byte) bool {
		props := make(map[string]interface{})

		if err := json.Unmarshal(payload, &props); err == nil {
			suite.T().Logf("event received: %v", props)

			return props["topic"] == fmt.Sprintf("%s/%s/things/twin/events/modified", namespace.Namespace, namespace.Name) &&
				props["path"] == fmt.Sprintf("/features/%s/properties/%s", featureName, propertyName) &&
				props["value"] == newValue

		} else {
			suite.T().Logf("error while waiting for event: %v", err)

			return false
		}
	})

	err = suite.sendDittoEvent(channel, msg)

	require.NoError(suite.T(), err)

	require.True(suite.T(), <-eventCh, "property changed event should be received")

	propertyURL := fmt.Sprintf("%s/properties/%s", suite.featureURL, propertyName)
	body, err := suite.doRequest("GET", propertyURL)
	require.NoError(suite.T(), err)

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
				if err := websocket.Message.Receive(ws, &payload); err == nil && check(payload) {
					resultCh <- true
					return
				} else {
					suite.T().Logf("error while waiting for WS message: %v", err)
				}
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
	url := fmt.Sprintf("ws://%s:%d/ws/2", suite.cfg.DittoHost, suite.cfg.DittoPort)
	cfg, err := websocket.NewConfig(url, url)
	if err != nil {
		return nil, err
	}

	auth := fmt.Sprintf("%s:%s", suite.cfg.DittoUser, suite.cfg.DittoPassword)
	enc := base64.StdEncoding.EncodeToString([]byte(auth))
	cfg.Header = http.Header{
		"Authorization": {"Basic " + enc},
	}

	return websocket.DialConfig(cfg)
}

func (suite *ConnectorSuite) doRequest(method string, url string) ([]byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(suite.cfg.DittoUser, suite.cfg.DittoPassword)
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

func getThingConfig(mqttClient mqtt.Client) (*thingConfig, error) {
	type result struct {
		cfg *thingConfig
		err error
	}

	ch := make(chan result)

	if token := mqttClient.Subscribe("edge/thing/response", 1, func(client mqtt.Client, message mqtt.Message) {
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
		return nil, fmt.Errorf("thing confing not received in %v", timeout)
	}
}