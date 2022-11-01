// Copyright (c) 2021 Contributors to the Eclipse Foundation
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

package routing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
)

const (
	// TopicGwParamsRequest defines the thing's data request topic.
	TopicGwParamsRequest = "edge/thing/request"
	// TopicGwParamsResponse defines the thing's data response topic.
	TopicGwParamsResponse = "edge/thing/response"

	// TopicConnectionStatus defines the connection status message topic.
	TopicConnectionStatus = "edge/connection/remote/status"
)

const (
	// StatusConfigError defines the congifuration error status.
	StatusConfigError = "CONFIG_ERROR"

	// StatusProvisioningError defines a provisioning error status.
	StatusProvisioningError = "PROVISIONING_ERROR"
	// StatusProvisioningUpdate defines a running provisioning update status.
	StatusProvisioningUpdate = "PROVISIONING_UPDATE"
	// StatusProvisioningMissing defines a missing provisioning status.
	StatusProvisioningMissing = "PROVISIONING_MISSING"

	// StatusConnectionClosed defines the connection closes status.
	StatusConnectionClosed = "CONNECTION_CLOSED"
	// StatusConnectionUnknown defines the unknown connection status.
	StatusConnectionUnknown = "CONNECTION_UNKNOWN"
	// StatusConnectionError defines a connection error.
	StatusConnectionError = "CONNECTION_ERROR"
	// StatusConnectionNotAuthenticated defines a not authenticated connection status.
	StatusConnectionNotAuthenticated = "CONNECTION_NOT_AUTHENTICATED"
)

// CloudConnectionHandler defines a connection handler.
type CloudConnectionHandler struct {
	CloudClient *connector.MQTTConnection
	Logger      watermill.LoggerAdapter
}

// Connected is called to notify CloudConnectionHandler for connection status change.
func (h *CloudConnectionHandler) Connected(connected bool, err error) {
	if connected {
		h.CloudClient.Connect()
	} else {
		h.CloudClient.Disconnect()
	}
}

// GwParams defines the GW parameters.
type GwParams struct {
	DeviceID string `json:"deviceId"`
	TenantID string `json:"tenantId"`
	PolicyID string `json:"policyId,omitempty"`
}

// NewGwParams creates GWParams.
func NewGwParams(deviceID, tenantID, policyID string) *GwParams {
	return &GwParams{
		DeviceID: deviceID,
		TenantID: tenantID,
		PolicyID: policyID,
	}
}

// NewGwParamsMessage creates GWParams message.
func NewGwParamsMessage(params *GwParams) *message.Message {
	payload, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}
	return message.NewMessage(watermill.NewUUID(), payload)
}

// ReconnectHandler defines reconnection handler.
type ReconnectHandler struct {
	Pub message.Publisher

	Params *GwParams

	Logger logger.Logger
}

// Connected is called to notify ReconnectHandler for connection status change.
func (h *ReconnectHandler) Connected(connected bool, err error) {
	if connected {
		SendGwParams(h.Params, false, h.Pub, h.Logger)
	}
}

// ConnectionStatus defines a connection status data.
type ConnectionStatus struct {
	Connected bool   `json:"connected"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Cause     string `json:"cause,omitempty"`
}

// ErrorsHandler represents an error handler.
type ErrorsHandler struct {
	StatusPub message.Publisher

	Logger logger.Logger
}

// Connected is called to notify ErrorsHandler for connection status change.
func (h *ErrorsHandler) Connected(connected bool, err error) {
	if !connected && err != nil {
		if errors.Is(err, packets.ErrorRefusedBadUsernameOrPassword) {
			SendStatus(StatusConnectionNotAuthenticated, h.StatusPub, h.Logger)
		} else {
			SendStatus(StatusConnectionError, h.StatusPub, h.Logger)
		}
	}
}

// ConnectionStatusHandler represents a connection status handler.
type ConnectionStatusHandler struct {
	Pub message.Publisher

	Logger logger.Logger
}

// Connected is called to notify ConnectionStatusHandler for connection status change.
func (h *ConnectionStatusHandler) Connected(connected bool, err error) {
	if connected {
		SendStatus("", h.Pub, h.Logger)
	}
}

// SendStatus publishes a connection status.
func SendStatus(cause string, statusPub message.Publisher, logger logger.Logger) {
	status := &ConnectionStatus{
		Cause:     cause,
		Connected: len(cause) == 0,
		Timestamp: time.Now().Unix(),
	}

	if payload, err := json.Marshal(status); err == nil {
		message := message.NewMessage(watermill.NewUUID(), payload)
		message.SetContext(connector.SetRetainToCtx(message.Context(), true))
		if err := statusPub.Publish(TopicConnectionStatus, message); err != nil {
			logger.Errorf("Failed to publish connection status %#v: %v", status, err)
		} else {
			logger.Infof("Connection status %s", string(payload))
		}
	}
}

// SendGwParams publishes the GwParams.
func SendGwParams(params *GwParams, retain bool, pub message.Publisher, logger logger.Logger) {
	paramsMsg := NewGwParamsMessage(params)
	logger.Infof("Config parameters %s", string(paramsMsg.Payload))

	paramsMsg.SetContext(connector.SetRetainToCtx(paramsMsg.Context(), retain))

	if err := pub.Publish(TopicGwParamsResponse, paramsMsg); err != nil {
		logger.Errorf("Failed to publish config parameters %#v: %v", params, err)
	}
}

// ParamsBus creates the GW parametes bus.
func ParamsBus(router *message.Router,
	params *GwParams,
	mosquittoPub message.Publisher,
	mosquittoSub message.Subscriber,
	logger logger.Logger,
) *message.Handler {
	return router.AddHandler("params_bus",
		TopicGwParamsRequest,
		mosquittoSub,
		TopicGwParamsResponse,
		mosquittoPub,
		func(msg *message.Message) ([]*message.Message, error) {
			paramsMsg := NewGwParamsMessage(params)
			logger.Infof("Config parameters %s", string(paramsMsg.Payload))

			return []*message.Message{paramsMsg}, nil
		},
	)
}

// CreateServiceRouter creates the service router.
func CreateServiceRouter(localClient *connector.MQTTConnection,
	manager connector.SubscriptionManager,
	logger watermill.LoggerAdapter,
) *message.Router {

	router, _ := message.NewRouter(message.RouterConfig{}, logger)
	CreateLogHandler(router, manager, localClient)

	go func() {
		if err := router.Run(context.Background()); err != nil {
			logger.Error("Failed to run local services router", err, nil)
		}
	}()

	return router
}

// StatusCause defines the connection error status on MQTT connection error.
func StatusCause(err error) (string, bool) {
	if errors.Is(err, packets.ErrorRefusedIDRejected) {
		return StatusConnectionError, false
	}

	if errors.Is(err, packets.ErrorRefusedBadProtocolVersion) {
		return StatusConnectionError, false
	}

	if errors.Is(err, packets.ErrorRefusedNotAuthorised) {
		return StatusConnectionError, false
	}

	if errors.Is(err, packets.ErrorRefusedBadUsernameOrPassword) {
		return StatusConnectionNotAuthenticated, false
	}

	return StatusConnectionError, true
}
