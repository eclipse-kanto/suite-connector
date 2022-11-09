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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

	TimeDeltaMs int `def:"0"`
	TimeSleepMs int `def:"2000"`
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

	firstTime := true
	sleepDuration := time.Duration(suite.cfg.TimeSleepMs * int(time.Millisecond))
	for {
		if !firstTime {
			time.Sleep(sleepDuration)
		}
		firstTime = false
		statusURL := fmt.Sprintf("%s/features/ConnectionStatus/properties/status", suite.thingURL)
		body, err := suite.doRequest(http.MethodGet, statusURL)
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

func MillisToDuration(millis int) time.Duration {
	return time.Duration(millis * int(time.Millisecond))
}

func (suite *ConnectorSuite) TestCommand() {
	ws, err := suite.newWSConnection()
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	commandResponseCh := make(chan ProcessWSMessageResult)

	dittoHandler := func(requestID string, msg *protocol.Envelope) {
		if msg.Path == fmt.Sprintf("/features/%s/inbox/messages/%s", featureID, commandName) {
			value, ok := msg.Value.(string)
			if !ok {
				commandResponseCh <- ProcessWSMessageResult{false, fmt.Errorf("unexpected message payload: %v, %T", msg.Value, msg.Value)}
				return
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
			if err := suite.dittoClient.Reply(requestID, responseMsg); err != nil {
				commandResponseCh <- ProcessWSMessageResult{false, fmt.Errorf("failed to send response: %v", err)}
				return
			}
			commandResponseCh <- ProcessWSMessageResult{true, nil}
		}
	}
	suite.dittoClient.Subscribe(dittoHandler)
	defer suite.dittoClient.Unsubscribe(dittoHandler)

	correlationID := uuid.New().String()
	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmdMsgEnvelope := things.NewMessage(namespace).
		WithPayload("request").
		Feature(featureID).
		Inbox(commandName).Envelope(
		protocol.WithCorrelationID(correlationID),
		protocol.WithContentType("text/plain"))

	checkEqual := func(expected, actual interface{}, name string) error {
		if !assert.ObjectsAreEqual(expected, actual) {
			return fmt.Errorf("%s: expected: %v, actual %v", name, expected, actual)
		}
		return nil
	}

	err = websocket.JSON.Send(ws, cmdMsgEnvelope)
	require.NoError(suite.T(), err, "unable to send command to the backend via websocket")

	timeout := MillisToDuration(suite.cfg.EventTimeoutMs)

	// Check the sending of the response from the feature to the backend
	var responseSentResult ProcessWSMessageResult
	select {
	case result := <-commandResponseCh:
		responseSentResult = result
	case <-time.After(timeout):
		responseSentResult.Err = errors.New("timeout")
	}
	require.Equal(
		suite.T(),
		ProcessWSMessageResult{true, nil},
		responseSentResult,
		"command should be received and response should be sent")

	// Check the response from the feature to the backend
	result := ProcessWSMessages(timeout, ws, func(respMsg *protocol.Envelope) ProcessWSMessageResult {
		expectedPath := strings.ReplaceAll(cmdMsgEnvelope.Path, "inbox", "outbox")
		if err := checkEqual(expectedPath, respMsg.Path, "path"); err != nil {
			return ProcessWSMessageResult{false, err}
		}
		if err := checkEqual(*cmdMsgEnvelope.Topic, *respMsg.Topic, "topic"); err != nil {
			return ProcessWSMessageResult{false, err}
		}
		if err := checkEqual(respMsg.Status, http.StatusOK, "http ok"); err != nil {
			return ProcessWSMessageResult{false, err}
		}
		if err := checkEqual(fmt.Sprintf(responsePayloadTemplate, cmdMsgEnvelope.Value.(string)), respMsg.Value, "response payload"); err != nil {
			return ProcessWSMessageResult{false, err}
		}
		return ProcessWSMessageResult{true, nil}
	})
	require.Equal(suite.T(), ProcessWSMessageResult{true, nil}, result, "command response should be received")
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

	sub := fmt.Sprintf("START-SEND-EVENTS?filter=like(resource:path,'/features/%s/*')", featureID)
	err = websocket.Message.Send(ws, sub)
	require.NoError(suite.T(), err, "unable to listen for events by using a websocket connection")

	const subAck = "START-SEND-EVENTS:ACK"
	timeout := MillisToDuration(suite.cfg.EventTimeoutMs)

	require.NoError(suite.T(), WaitForAck(timeout, ws, subAck), "acknowledgement %v should be received", subAck)

	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(namespace).Twin().
		FeatureProperty(featureID, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = suite.sendDittoEvent(channel, msg)

	require.NoError(suite.T(), err, "unable to send event to the backend")

	result := ProcessWSMessages(timeout, ws, func(msg *protocol.Envelope) ProcessWSMessageResult {
		suite.T().Logf("event received: %v", msg)
		if msg.Topic.String() == fmt.Sprintf("%s/%s/things/twin/events/modified", namespace.Namespace, namespace.Name) &&
			msg.Path == fmt.Sprintf("/features/%s/properties/%s", featureID, propertyName) &&
			msg.Value == newValue {
			return ProcessWSMessageResult{true, nil}
		}
		return ProcessWSMessageResult{false, fmt.Errorf("unexpected value: %s", msg.Value)}
	})
	require.Equal(suite.T(), ProcessWSMessageResult{true, nil}, result, "property changed event should be received")

	propertyURL := fmt.Sprintf("%s/properties/%s", suite.featureURL, propertyName)
	body, err := suite.doRequest(http.MethodGet, propertyURL)
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

func WaitForAck(
	timeout time.Duration,
	ws *websocket.Conn,
	expectedAck string) error {

	var payload []byte
	deadline := time.Now().Add(timeout)
	ws.SetDeadline(deadline)

	for time.Now().Before(deadline) {
		err := websocket.Message.Receive(ws, &payload)
		if err != nil {
			return fmt.Errorf("error reading from websocket: %s", err)
		}
		ack := strings.TrimSpace(string(payload))
		if ack == expectedAck {
			return nil
		}
	}
	return errors.New("timeout")
}

type ProcessWSMessageResult struct {
	Finished bool
	Err      error
}

func ProcessWSMessages(
	timeout time.Duration,
	ws *websocket.Conn,
	process func(*protocol.Envelope) ProcessWSMessageResult) ProcessWSMessageResult {
	deadline := time.Now().Add(timeout)
	ws.SetDeadline(deadline)

	result := ProcessWSMessageResult{}
	for !result.Finished && time.Now().Before(deadline) {
		var payload []byte
		webSocketErr := websocket.Message.Receive(ws, &payload)
		if webSocketErr != nil {
			result.Err = fmt.Errorf("error reading from websocket: %s", webSocketErr)
			return result
		}
		envelope := &protocol.Envelope{}
		unmarshalErr := json.Unmarshal(payload, envelope)
		if unmarshalErr == nil {
			result = process(envelope)
		} else {
			// Unmarshalling error, the payload is not a JSON of protocol.Envelope
			// Ignore the error
			fmt.Fprintf(os.Stderr, "error unmarshalling a protocol.Envelope: %v", unmarshalErr)
		}
	}

	if !result.Finished {
		result.Err = fmt.Errorf("not finished, expected WS response not received in %v, last error: %v", timeout, result.Err)
	}
	return result
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

	token := mqttClient.Subscribe("edge/thing/response", 1, func(client MQTT.Client, message MQTT.Message) {
		var cfg thingConfig
		if err := json.Unmarshal(message.Payload(), &cfg); err != nil {
			ch <- result{nil, err}
		}
		ch <- result{&cfg, nil}
	})
	defer mqttClient.Unsubscribe("edge/thing/response")

	if token.Wait() && token.Error() != nil {
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
