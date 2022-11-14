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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	env "github.com/caarlos0/env/v6"

	"github.com/eclipse-kanto/kanto/integration/util"
	"github.com/eclipse/ditto-clients-golang/model"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/eclipse/ditto-clients-golang/protocol/things"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/websocket"
)

type suiteConnectorTestConfig struct {
	StatusTimeoutMs int `env:"SCT_STATUS_TIMEOUT_MS" envDefault:"10000"`

	TimeDeltaMs int `env:"SCT_TIME_DELTA_MS" envDefault:"0"`
	TimeSleepMs int `env:"SCT_TIME_SLEEP_MS" envDefault:"2000"`
}

type ConnectorSuite struct {
	suite.Suite
	util.SuiteInitializer

	thingCfg   *util.ThingConfiguration
	testConfig *suiteConnectorTestConfig

	thingURL   string
	featureURL string
}

const (
	featureID               = "ConnectorTestFeature"
	propertyName            = "testProperty"
	commandName             = "testCommand"
	responsePayloadTemplate = "responsePayload: %s"
)

func (suite *ConnectorSuite) SetupSuite() {
	suite.Setup(suite.T())

	cfg := suite.Cfg

	testConfig := &suiteConnectorTestConfig{}
	opts := env.Options{RequiredIfNoDef: true}
	require.NoError(suite.T(), env.Parse(testConfig, opts), "Failed to process environment variables")

	thingCfg, err := util.GetThingConfiguration(cfg, suite.MQTTClient)
	require.NoError(suite.T(), err, "init thing cfg")

	feature := &model.Feature{}
	feature.WithProperty(propertyName, "testValue")

	cmd := things.NewCommand(model.NewNamespacedIDFrom(thingCfg.DeviceID)).Twin().Feature(featureID).
		Modify(feature)
	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = suite.DittoClient.Send(msg)
	require.NoError(suite.T(), err, "create test feature")

	suite.thingCfg = thingCfg
	suite.testConfig = testConfig
	suite.thingURL = fmt.Sprintf("%s/api/2/things/%s", strings.TrimSuffix(cfg.DigitalTwinAPIAddress, "/"), thingCfg.DeviceID)
	suite.featureURL = fmt.Sprintf("%s/features/%s", suite.thingURL, featureID)
}

func (suite *ConnectorSuite) TearDownSuite() {
	if _, err := util.SendDigitalTwinRequest(suite.Cfg, http.MethodDelete, suite.featureURL, nil); err != nil {
		suite.T().Logf("error while deleting test feature: %v", err)
	}
	suite.TearDown()
}

func TestConnectorSuite(t *testing.T) {
	suite.Run(t, new(ConnectorSuite))
}

func (suite *ConnectorSuite) TestConnectionStatus() {
	type connectionStatus struct {
		ReadySince time.Time `json:"readySince"`
		ReadyUntil time.Time `json:"readyUntil"`
	}

	cfg := suite.Cfg
	testConfig := suite.testConfig

	timeout := util.MillisToDuration(testConfig.StatusTimeoutMs)
	threshold := time.Now().Add(timeout)

	firstTime := true
	sleepDuration := util.MillisToDuration(testConfig.TimeSleepMs)
	for {
		if !firstTime {
			time.Sleep(sleepDuration)
		}
		firstTime = false
		statusURL := fmt.Sprintf("%s/features/ConnectionStatus/properties/status", suite.thingURL)
		body, err := util.SendDigitalTwinRequest(cfg, http.MethodGet, statusURL, nil)
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

		delta := int64(testConfig.TimeDeltaMs)
		assert.Less(suite.T(), status.ReadySince.UnixMilli(), time.Now().UnixMilli()+delta, "readySince should be before current time")
		break
	}
}

func (suite *ConnectorSuite) TestCommand() {
	cfg := suite.Cfg

	ws, err := util.NewDigitalTwinWSConnection(cfg)
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	commandResponseCh := make(chan error)

	dittoHandler := func(requestID string, msg *protocol.Envelope) {
		if msg.Path == fmt.Sprintf("/features/%s/inbox/messages/%s", featureID, commandName) {
			value, ok := msg.Value.(string)
			if !ok {
				commandResponseCh <- fmt.Errorf("unexpected message payload: %v, %T", msg.Value, msg.Value)
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
			if err := suite.DittoClient.Reply(requestID, responseMsg); err != nil {
				commandResponseCh <- fmt.Errorf("failed to send response: %v", err)
				return
			}
			commandResponseCh <- nil
		}
	}
	suite.DittoClient.Subscribe(dittoHandler)
	defer suite.DittoClient.Unsubscribe(dittoHandler)

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

	timeout := util.MillisToDuration(cfg.WsEventTimeoutMs)

	// Check the sending of the response from the feature to the backend
	var responseSentResult error
	select {
	case result := <-commandResponseCh:
		responseSentResult = result
	case <-time.After(timeout):
		responseSentResult = errors.New("receiving command and sending response timed out")
	}

	require.NoError(suite.T(),
		responseSentResult,
		"command should be received and response should be sent")

	// Check the response from the feature to the backend
	result := util.ProcessWSMessages(cfg, ws, func(respMsg *protocol.Envelope) (bool, error) {
		expectedPath := strings.ReplaceAll(cmdMsgEnvelope.Path, "inbox", "outbox")
		if err := checkEqual(expectedPath, respMsg.Path, "path"); err != nil {
			return false, err
		}
		if err := checkEqual(*cmdMsgEnvelope.Topic, *respMsg.Topic, "topic"); err != nil {
			return false, err
		}
		if err := checkEqual(respMsg.Status, http.StatusOK, "http ok"); err != nil {
			return false, err
		}
		if err := checkEqual(fmt.Sprintf(responsePayloadTemplate, cmdMsgEnvelope.Value.(string)), respMsg.Value, "response payload"); err != nil {
			return false, err
		}
		return true, nil
	})
	require.NoError(suite.T(), result, "command response should be received")
}

func (suite *ConnectorSuite) TestEvent() {
	suite.testModify("e", "testEvent")
}

func (suite *ConnectorSuite) TestTelemetry() {
	suite.testModify("t", "testTelemetry")
}

func (suite *ConnectorSuite) testModify(topic string, newValue string) {
	cfg := suite.Cfg
	ws, err := util.NewDigitalTwinWSConnection(cfg)
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	sub := fmt.Sprintf("START-SEND-EVENTS?filter=like(resource:path,'/features/%s/*')", featureID)
	err = websocket.Message.Send(ws, sub)
	require.NoError(suite.T(), err, "unable to listen for events by using a websocket connection")

	const subAck = "START-SEND-EVENTS:ACK"

	require.NoError(suite.T(), util.WaitForWSMessage(cfg, ws, subAck), "acknowledgement %v should be received", subAck)

	namespace := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(namespace).Twin().
		FeatureProperty(featureID, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = util.SendMQTTMessage(suite.Cfg, suite.MQTTClient, topic, msg)

	require.NoError(suite.T(), err, "unable to send event to the backend")

	result := util.ProcessWSMessages(cfg, ws, func(msg *protocol.Envelope) (bool, error) {
		if msg.Topic.String() == fmt.Sprintf("%s/%s/things/twin/events/modified", namespace.Namespace, namespace.Name) &&
			msg.Path == fmt.Sprintf("/features/%s/properties/%s", featureID, propertyName) &&
			msg.Value == newValue {
			return true, nil
		}
		return false, fmt.Errorf("unexpected value: %s", msg.Value)
	})
	require.NoError(suite.T(), result, "property changed event should be received")

	propertyURL := fmt.Sprintf("%s/properties/%s", suite.featureURL, propertyName)
	body, err := util.SendDigitalTwinRequest(cfg, http.MethodGet, propertyURL, nil)
	require.NoError(suite.T(), err, "unable to get feature property")

	assert.Equal(suite.T(), fmt.Sprintf("\"%s\"", newValue), strings.TrimSpace(string(body)), "property value updated")
}
