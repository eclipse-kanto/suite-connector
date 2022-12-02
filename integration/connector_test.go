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
)

type connectorTestConfiguration struct {
	StatusTimeoutMs             int `env:"SCT_STATUS_TIMEOUT_MS" envDefault:"10000"`
	StatusReadySinceTimeDeltaMs int `env:"SCT_STATUS_READY_SINCE_TIME_DELTA_MS" envDefault:"0"`
	StatusRetryIntervalMs       int `env:"SCT_STATUS_RETRY_INTERVAL_MS" envDefault:"2000"`
}

type connectorSuite struct {
	suite.Suite
	util.SuiteInitializer

	connectorTestCfg *connectorTestConfiguration

	thingURL   string
	featureURL string
}

const (
	featureID               = "ConnectorTestFeature"
	propertyName            = "testProperty"
	commandName             = "testCommand"
	commandPayload          = "request"
	responsePayloadTemplate = "responsePayload: %s"
)

func (suite *connectorSuite) SetupSuite() {
	suite.Setup(suite.T())

	connectorTestCfg := &connectorTestConfiguration{}
	opts := env.Options{RequiredIfNoDef: true}
	require.NoError(suite.T(), env.Parse(connectorTestCfg, opts),
		"failed to process suite connector test environment variables")

	feature := &model.Feature{}
	feature.WithProperty(propertyName, "testValue")

	cmd := things.NewCommand(model.NewNamespacedIDFrom(suite.ThingCfg.DeviceID)).Twin().Feature(featureID).
		Modify(feature)
	msg := cmd.Envelope(protocol.WithResponseRequired(false))
	err := suite.DittoClient.Send(msg)
	require.NoError(suite.T(), err, "create test feature")

	suite.connectorTestCfg = connectorTestCfg
	suite.thingURL = util.GetThingURL(suite.Cfg.DigitalTwinAPIAddress, suite.ThingCfg.DeviceID)
	suite.featureURL = util.GetFeatureURL(suite.thingURL, featureID)
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

		body, err := util.GetFeaturePropertyValue(suite.Cfg,
			util.GetFeatureURL(suite.thingURL, "ConnectionStatus"), "status")
		now := time.Now()
		if err != nil {
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
		if !forever.Equal(status.ReadyUntil) {
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
	// Minimize the scope of this utility function
	responsePayload := func(value string) string {
		return fmt.Sprintf(responsePayloadTemplate, value)
	}

	dittoHandler := func(requestID string, msg *protocol.Envelope) {
		featureInbox := util.GetFeatureInboxMessagePath(featureID, commandName)
		if msg.Path == featureInbox {
			value, ok := msg.Value.(string)
			if !ok {
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
			suite.DittoClient.Reply(requestID, responseMsg)
		}
	}
	suite.DittoClient.Subscribe(dittoHandler)
	defer suite.DittoClient.Unsubscribe(dittoHandler)

	body, err := util.ExecuteOperation(suite.Cfg, suite.featureURL, commandName, commandPayload)
	require.NoError(suite.T(), err, "sending the command via REST and receiving response should work")
	expectedResponse := responsePayload(commandPayload)
	require.Equal(suite.T(), expectedResponse, string(body), "command response should match expected")
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

	filter := fmt.Sprintf("like(resource:path,'/features/%s/*')", featureID)
	err = util.SubscribeForWSMessages(suite.Cfg, ws, util.StartSendEvents, filter)
	require.NoError(suite.T(), err, "subscription for events should succeed")
	defer util.UnsubscribeFromWSMessages(suite.Cfg, ws, util.StopSendEvents)

	thingID := model.NewNamespacedIDFrom(suite.ThingCfg.DeviceID)
	cmd := things.NewCommand(thingID).Twin().
		FeatureProperty(featureID, propertyName).Modify(newValue)

	msg := cmd.Envelope(protocol.WithResponseRequired(false))
	err = util.SendMQTTMessage(suite.Cfg, suite.MQTTClient, topic, msg)
	require.NoError(suite.T(), err, "unable to send event to the backend")

	result := util.ProcessWSMessages(suite.Cfg, ws, func(msg *protocol.Envelope) (bool, error) {
		expectedTopic := util.GetTwinEventTopic(suite.ThingCfg.DeviceID, protocol.ActionModified)
		expectedPath := util.GetFeaturePropertyPath(featureID, propertyName)
		if expectedTopic == msg.Topic.String() &&
			expectedPath == msg.Path &&
			msg.Value == newValue {
			return true, nil
		}
		return false, fmt.Errorf("unexpected value: %s", msg.Value)
	})
	require.NoError(suite.T(), result, "property changed event should be received")

	body, err := util.GetFeaturePropertyValue(suite.Cfg, suite.featureURL, propertyName)
	require.NoError(suite.T(), err, "unable to get feature property")

	assert.Equal(suite.T(), fmt.Sprintf("\"%s\"", newValue), strings.TrimSpace(string(body)), "property value updated")
}
