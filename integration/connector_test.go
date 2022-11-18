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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/websocket"
)

type connectorTestConfiguration struct {
	CommandTimeoutMs int `env:"SCT_COMMAND_TIMEOUT_MS" envDefault:"30000"`

	StatusTimeoutMs             int `env:"SCT_STATUS_TIMEOUT_MS" envDefault:"10000"`
	StatusReadySinceTimeDeltaMs int `env:"SCT_STATUS_READY_SINCE_TIME_DELTA_MS" envDefault:"0"`
	StatusRetryIntervalMs       int `env:"SCT_STATUS_RETRY_INTERVAL_MS" envDefault:"2000"`
}

type connectorSuite struct {
	suite.Suite
	util.SuiteInitializer

	thingCfg         *util.ThingConfiguration
	connectorTestCfg *connectorTestConfiguration

	thingURL            string
	featureURL          string
	featurePropertyPath string
}

const (
	featureID               = "ConnectorTestFeature"
	propertyName            = "testProperty"
	commandName             = "testCommand"
	responsePayloadTemplate = "responsePayload: %s"
)

func (suite *connectorSuite) SetupSuite() {
	suite.Setup(suite.T())

	connectorTestCfg := &connectorTestConfiguration{}
	opts := env.Options{RequiredIfNoDef: true}
	require.NoError(suite.T(), env.Parse(connectorTestCfg, opts),
		"failed to process suite connector test environment variables")

	thingCfg, err := util.GetThingConfiguration(suite.Cfg, suite.MQTTClient)
	require.NoError(suite.T(), err, "init thing cfg")

	feature := &model.Feature{}
	feature.WithProperty(propertyName, "testValue")

	cmd := things.NewCommand(model.NewNamespacedIDFrom(thingCfg.DeviceID)).Twin().Feature(featureID).
		Modify(feature)
	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = suite.DittoClient.Send(msg)
	require.NoError(suite.T(), err, "create test feature")

	suite.thingCfg = thingCfg
	suite.connectorTestCfg = connectorTestCfg
	suite.thingURL = fmt.Sprintf("%s/api/2/things/%s",
		strings.TrimSuffix(suite.Cfg.DigitalTwinAPIAddress, "/"), thingCfg.DeviceID)
	suite.featureURL = fmt.Sprintf("%s/features/%s", suite.thingURL, featureID)
	suite.featurePropertyPath = fmt.Sprintf("/features/%s/properties/%s", featureID, propertyName)
}

func (suite *connectorSuite) TearDownSuite() {
	if _, err := util.SendDigitalTwinRequest(suite.Cfg, http.MethodDelete, suite.featureURL, nil); err != nil {
		suite.T().Logf("error while deleting test feature: %v", err)
	}
	suite.TearDown()
}

func TestConnectorSuite(t *testing.T) {
	suite.Run(t, new(connectorSuite))
}

func (suite *connectorSuite) TestConnectionStatus() {
	type connectionStatus struct {
		ReadySince time.Time `json:"readySince"`
		ReadyUntil time.Time `json:"readyUntil"`
	}

	timeout := util.MillisToDuration(suite.connectorTestCfg.StatusTimeoutMs)
	threshold := time.Now().Add(timeout)

	firstTime := true
	sleepDuration := util.MillisToDuration(suite.connectorTestCfg.StatusRetryIntervalMs)
	for {
		if !firstTime {
			time.Sleep(sleepDuration)
		}
		firstTime = false
		statusURL := fmt.Sprintf("%s/features/ConnectionStatus/properties/status", suite.thingURL)
		body, err := util.SendDigitalTwinRequest(suite.Cfg, http.MethodGet, statusURL, nil)
		var now time.Time
		if err != nil {
			now = time.Now()
			if now.Before(threshold) {
				continue
			}
			suite.T().Errorf("connection status property not available: %v", err)
			break
		}

		status := &connectionStatus{}
		err = json.Unmarshal(body, status)
		require.NoError(suite.T(), err, "connection status should be parsed")

		suite.T().Logf("%+v", status)
		suite.T().Logf("current time: %v", now)

		forever := time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
		if status.ReadyUntil != forever {
			if now.Before(threshold) {
				continue
			}
			suite.T().Errorf("readyUntil should be %v", forever)
			break
		}

		delta := int64(suite.connectorTestCfg.StatusReadySinceTimeDeltaMs)

		now = time.Now()
		assert.Less(suite.T(), status.ReadySince.UnixMilli(), now.UnixMilli()+delta, "readySince should be before current time")
		break
	}
}

func (suite *connectorSuite) TestCommand() {
	commandResponseCh := make(chan error)

	responsePayload := func(value string) string {
		return fmt.Sprintf(responsePayloadTemplate, value)
	}

	dittoHandler := func(requestID string, msg *protocol.Envelope) {
		if msg.Path == fmt.Sprintf("/features/%s/inbox/messages/%s", featureID, commandName) {
			value, ok := msg.Value.(string)
			if !ok {
				commandResponseCh <- fmt.Errorf("unexpected message payload: %v, %T", msg.Value, msg.Value)
				return
			}
			response := things.NewMessage(model.NewNamespacedID(msg.Topic.Namespace, msg.Topic.EntityName)).
				WithPayload(responsePayload(value)).Feature(featureID).Outbox(commandName)
			// respond to the message by using the outbox
			responseMsg := response.Envelope(
				protocol.WithCorrelationID(msg.Headers.CorrelationID()),
				protocol.WithResponseRequired(false),
				protocol.WithContentType("text/plain")).
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

	const commandPayload = "request"

	commandTimeoutSeconds := suite.connectorTestCfg.CommandTimeoutMs / 1000
	digitalTwinAPI := fmt.Sprintf("%s/inbox/messages/%s?timeout=%d", suite.featureURL, commandName, commandTimeoutSeconds)
	// This request blocks until the command has been proessed and the response has been received.
	// Run it in a goroutine, so we can process the digital twin API events while this is running.
	body, err := util.SendDigitalTwinRequest(suite.Cfg, http.MethodPost, digitalTwinAPI, commandPayload)
	require.NoError(suite.T(), err, "sending the command via REST and receiving response should work")
	expectedResponse := responsePayload(commandPayload)
	require.Equal(suite.T(), expectedResponse, string(body), "command response should match expected")

	// Check the sending of the response from the feature to the backend
	responseSentResult := waitWithTimeout(
		suite.connectorTestCfg.CommandTimeoutMs, commandResponseCh, "receiving command and sending response timed out")
	require.NoError(suite.T(),
		responseSentResult,
		"receiving command and sending response should succeed")
}

func waitWithTimeout(timeoutMS int, commandResponseCh chan error, timeoutErrorMessage string) error {
	timeout := util.MillisToDuration(timeoutMS)

	var responseSentResult error
	select {
	case result := <-commandResponseCh:
		responseSentResult = result
	case <-time.After(timeout):
		responseSentResult = errors.New(timeoutErrorMessage)
	}
	return responseSentResult
}

func (suite *connectorSuite) TestEvent() {
	suite.testModify("e", "testEvent")
}

func (suite *connectorSuite) TestTelemetry() {
	suite.testModify("t", "testTelemetry")
}

func (suite *connectorSuite) testModify(topic string, newValue string) {
	ws, err := util.NewDigitalTwinWSConnection(suite.Cfg)
	require.NoError(suite.T(), err, "cannot create a websocket connection to the backend")
	defer ws.Close()

	sub := fmt.Sprintf("START-SEND-EVENTS?filter=like(resource:path,'/features/%s/*')", featureID)
	err = websocket.Message.Send(ws, sub)
	require.NoError(suite.T(), err, "unable to listen for events by using a websocket connection")

	const subAck = "START-SEND-EVENTS:ACK"
	require.NoError(suite.T(), util.WaitForWSMessage(suite.Cfg, ws, subAck), "acknowledgement %v should be received", subAck)

	thingID := model.NewNamespacedIDFrom(suite.thingCfg.DeviceID)
	cmd := things.NewCommand(thingID).Twin().
		FeatureProperty(featureID, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))

	err = util.SendMQTTMessage(suite.Cfg, suite.MQTTClient, topic, msg)

	require.NoError(suite.T(), err, "unable to send event to the backend")

	result := util.ProcessWSMessages(suite.Cfg, ws, func(msg *protocol.Envelope) (bool, error) {
		expectedTopic := protocol.Topic{
			Namespace:  thingID.Namespace,
			EntityName: thingID.Name,
			Group:      protocol.GroupThings,
			Channel:    protocol.ChannelTwin,
			Criterion:  protocol.CriterionEvents,
			Action:     protocol.ActionModified,
		}
		if expectedTopic == *msg.Topic &&
			suite.featurePropertyPath == msg.Path &&
			msg.Value == newValue {
			return true, nil
		}
		return false, fmt.Errorf("unexpected value: %s", msg.Value)
	})
	require.NoError(suite.T(), result, "property changed event should be received")

	propertyURL := fmt.Sprintf("%s/properties/%s", suite.featureURL, propertyName)
	body, err := util.SendDigitalTwinRequest(suite.Cfg, http.MethodGet, propertyURL, nil)
	require.NoError(suite.T(), err, "unable to get feature property")

	assert.Equal(suite.T(), fmt.Sprintf("\"%s\"", newValue), strings.TrimSpace(string(body)), "property value updated")
}
