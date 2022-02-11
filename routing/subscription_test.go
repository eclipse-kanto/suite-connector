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

package routing_test

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/testutil"
	"github.com/eclipse-kanto/suite-connector/util"
)

func TestParsingSubscription(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogSubscribe,
		"1610290410: MQTT_FX_Client 0 command//<deviceId> part2 /req/#")
	assert.Equal(t, message.Type, routing.Subscription)
	assert.Equal(t, message.Text, "command//<deviceId> part2 /req/#")
}

func TestParsingWrongSubscription(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogSubscribe,
		"1610290410: MQTT_FX_Client 0 telemetry/#")
	assert.Nil(t, message)
}

func TestParsingUnSubscription(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogUnsubscribe,
		"1601396193: cmcosqsub|5677-user-Vir command//4711 /req/#")
	assert.Equal(t, message.Type, routing.UnSubscription)
	assert.Equal(t, message.Text, "command//4711 /req/#")

	message = routing.ParseLogMessage(
		routing.TopicLogUnsubscribe,
		"1601396193: cmcosqsub|5677-user-Vir c//4711 /q/#")
	assert.Equal(t, message.Type, routing.UnSubscription)
	assert.Equal(t, message.Text, "c//4711 /q/#")
}

func TestParsingDisconnection(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogNoticeLevel,
		"1610289865: Client MQTT_FX_Client disconnected.")
	assert.Equal(t, message.Type, routing.Disconnection)
}

func TestParsingTermination(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogNoticeLevel,
		"1610290521: Socket error on client MQTT_FX_Client, disconnecting.")
	assert.Equal(t, message.Type, routing.Termination)
}

func TestParsingTerminationMosquitto_2_x(t *testing.T) {
	message := routing.ParseLogMessage(
		routing.TopicLogNoticeLevel,
		"1612450735: Client MQTT_FX_Client closed its connection.")
	assert.Equal(t, message.Type, routing.Termination)
}

func TestSubscriptionItem(t *testing.T) {
	logMessage := routing.ParseLogMessage(
		routing.TopicLogUnsubscribe,
		"1601396193: cmcosqsub|5677-user-Vir command//4711 /req/#")
	require.Equal(t, logMessage.Type, routing.UnSubscription)

	topicNorm := util.NormalizeTopic(logMessage.Text)
	subscrItem := routing.SubscriptionItem{
		ClientID:  logMessage.ClientID,
		Timestamp: logMessage.Timestamp,
		TopicID:   topicNorm,
		TopicReal: logMessage.Text,
	}

	str := subscrItem.String()
	assert.NotEmpty(t, str)
	assert.True(t, strings.Contains(str, "clientId"), str)
	assert.True(t, strings.Contains(str, "timestamp"), str)
	assert.True(t, strings.Contains(str, "topicId"), str)
	assert.True(t, strings.Contains(str, "topicReal"), str)
}

func TestLogHandlerAccept(t *testing.T) {
	h := &routing.LogHandler{
		SubcriptionList: list.New(),
		Manager:         connector.NewSubscriptionManager(),
		Logger:          testutil.NewLogger("subscription", logger.INFO),
	}

	msg := message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy 0 command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
	msg = message.NewMessage(watermill.NewUUID(), []byte("1635966265: dummy command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogUnsubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	msg = message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy1 0 command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
	msg = message.NewMessage(watermill.NewUUID(), []byte("1612450735: Client dummy1 closed its connection."))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogNoticeLevel))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	msg = message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy2 0 command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
	msg = message.NewMessage(watermill.NewUUID(), []byte("1610289865: Client dummy2 disconnected."))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogNoticeLevel))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	msg = message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy3 0 command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
	msg = message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy4 0 command//Testing:Logs/req/#"))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	msg = message.NewMessage(watermill.NewUUID(), []byte("1610290521: Socket error on client dummy3, disconnecting."))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogNoticeLevel))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 1, h.SubcriptionList.Len())
}

func TestLogHandlerReject(t *testing.T) {
	h := &routing.LogHandler{
		SubcriptionList: list.New(),
		Manager:         connector.NewSubscriptionManager(),
		Logger:          testutil.NewLogger("subscription", logger.INFO),
	}

	msg := message.NewMessage(watermill.NewUUID(), []byte("1635966263: dummy 0 command//Testing:Logs/req/#"))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	localClientID := "connector"
	if localID := os.Getenv("LOCAL_CLIENTID"); len(localID) > 0 {
		localClientID = localID
	}

	subMsg := fmt.Sprintf("1635966263: %s 0 command//Testing:Logs/req/#", localClientID)
	msg = message.NewMessage(watermill.NewUUID(), []byte(subMsg))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogSubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	cloudClientID := "cloud"
	if cloudID := os.Getenv("CLOUD_CLIENTID"); len(cloudID) > 0 {
		cloudClientID = cloudID
	}

	unSubMsg := fmt.Sprintf("1635966265: %s command//Testing:Logs/req/#", cloudClientID)
	msg = message.NewMessage(watermill.NewUUID(), []byte(unSubMsg))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogUnsubscribe))
	assert.NoError(t, h.ProcessLogs(msg))
	assert.Equal(t, 0, h.SubcriptionList.Len())

	invalid := message.NewMessage(watermill.NewUUID(), []byte("1610290521: testing on client invalid."))
	invalid.SetContext(connector.SetTopicToCtx(context.Background(), routing.TopicLogNoticeLevel))
	assert.NoError(t, h.ProcessLogs(invalid))
}
