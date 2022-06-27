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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/cenkalti/backoff/v3"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/util"
	"github.com/eclipse/ditto-clients-golang/model"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

var (
	localClientID = "connector"
	cloudClientID = "cloud"
)

func init() {
	if localID := os.Getenv("LOCAL_CLIENTID"); len(localID) > 0 {
		localClientID = localID
	}

	if cloudID := os.Getenv("CLOUD_CLIENTID"); len(cloudID) > 0 {
		cloudClientID = cloudID
	}
}

// HubConnectionSettings contains remote connection related settings.
type HubConnectionSettings struct {
	DeviceID        string `json:"deviceId"`
	DeviceIDPattern string `json:"deviceIdPattern"`

	TenantID string `json:"tenantId"`
	PolicyID string `json:"policyId"`
	Address  string `json:"address"`
	Password string `json:"password"`
	ClientID string `json:"clientId"`
	AuthID   string `json:"authId"`

	UseCertificate          bool `json:"-"`
	AutoProvisioningEnabled bool `json:"-"`

	TLSSettings
}

// Validate validates the hub connection settings.
func (c *HubConnectionSettings) Validate() error {
	if len(c.DeviceID) == 0 && len(c.DeviceIDPattern) == 0 {
		return errors.New("device ID or pattern is missing")
	}

	if len(c.DeviceID) > 0 {
		if ns := model.NewNamespacedIDFrom(c.DeviceID); ns == nil {
			return errors.Errorf("device ID '%s' has invalid namespace ID", c.DeviceID)
		}
	}

	if len(c.TenantID) == 0 {
		return errors.New("tenant ID is missing")
	}

	if len(c.Address) == 0 {
		return errors.New("remote broker address is missing")
	}

	return nil
}

// LocalConnectionSettings contains the local connection settings.
type LocalConnectionSettings struct {
	LocalAddress  string `json:"localAddress"`
	LocalUsername string `json:"localUsername"`
	LocalPassword string `json:"localPassword"`

	LocalCACert string `json:"localCACert"`
	LocalCert   string `json:"localCert"`
	LocalKey    string `json:"localKey"`
}

// Validate validates the local connection settings.
func (c *LocalConnectionSettings) Validate() error {
	if len(c.LocalAddress) == 0 {
		return errors.New("local address is missing")
	}
	return nil
}

// CreateHubConnection creates a hub connection.
func CreateHubConnection(
	settings *HubConnectionSettings,
	nopStore bool,
	logger watermill.LoggerAdapter,
) (*conn.MQTTConnection, Cleaner, error) {
	honoConfig, err := conn.NewMQTTClientConfig(settings.Address)
	if err != nil {
		return nil, nil, err
	}

	u, err := url.Parse(honoConfig.URL)
	if err != nil {
		return nil, nil, err
	}

	honoConfig.CleanSession = true
	honoConfig.ExternalReconnect = true
	honoConfig.NoOpStore = nopStore
	honoConfig.ConnectRetryInterval = 0

	honoConfig.BackoffMultiplier = 2
	honoConfig.MinReconnectInterval = time.Minute
	honoConfig.MaxReconnectInterval = 4 * time.Minute

	if min, err := strconv.ParseInt(os.Getenv("HUB_CONNNECT_INIT"), 0, 64); err == nil {
		honoConfig.MinReconnectInterval = time.Duration(min) * time.Second
	}

	if max, err := strconv.ParseInt(os.Getenv("HUB_CONNNECT_MAX"), 0, 64); err == nil {
		honoConfig.MaxReconnectInterval = time.Duration(max) * time.Second
	}

	if mul, err := strconv.ParseFloat(os.Getenv("HUB_CONNNECT_MUL"), 32); err == nil {
		honoConfig.BackoffMultiplier = mul
	}

	tlsConfig, cleaner, err := NewHubTLSConfig(&settings.TLSSettings, logger)
	if err != nil {
		return nil, nil, err
	}

	if len(tlsConfig.Certificates) == 0 {
		settings.UseCertificate = false
		honoConfig.Credentials.UserName = util.NewHonoUserName(settings.AuthID, settings.TenantID)
		honoConfig.Credentials.Password = settings.Password

	} else {
		if len(settings.DeviceIDPattern) > 0 {
			resolved, err := util.ReplacePattern(settings.DeviceIDPattern, tlsConfig.Certificates[0])
			if err != nil {
				defer cleaner()
				return nil, nil, errors.Wrapf(err, "unable to resolve device ID pattern '%s'", settings.DeviceIDPattern)
			}

			if ns := model.NewNamespacedIDFrom(resolved); ns == nil {
				defer cleaner()
				return nil, nil, errors.Errorf("device ID '%s' has invalid namespace ID", resolved)
			}

			settings.DeviceID = resolved
		}
	}

	switch u.Scheme {
	case "wss", "ssl", "tls", "mqtts", "mqtt+ssl", "tcps":
		honoConfig.TLSConfig = tlsConfig
	default:
		// unsupported
	}

	conn, err := conn.NewMQTTConnection(honoConfig, settings.ClientID, logger)
	if err != nil {
		defer cleaner()

		return nil, nil, err
	}

	return conn, cleaner, err
}

// CreateLocalConnection creates a local mosquitto connection.
func CreateLocalConnection(
	settings *LocalConnectionSettings,
	logger watermill.LoggerAdapter,
) (*conn.MQTTConnection, error) {
	mosquittoConfig, err := conn.NewMQTTClientConfig(settings.LocalAddress)
	if err != nil {
		return nil, err
	}

	statusBean := &routing.ConnectionStatus{
		Cause: routing.StatusConnectionUnknown,
	}

	status, err := json.Marshal(statusBean)
	if err != nil {
		return nil, err
	}

	mosquittoConfig.CleanSession = true
	mosquittoConfig.WillTopic = routing.TopicConnectionStatus
	mosquittoConfig.WillQos = conn.QosAtLeastOnce
	mosquittoConfig.WillRetain = true
	mosquittoConfig.WillMessage = status
	mosquittoConfig.ConnectRetryInterval = 0
	mosquittoConfig.MinReconnectInterval = 5 * time.Second
	mosquittoConfig.MaxReconnectInterval = 5 * time.Second

	if len(settings.LocalUsername) > 0 {
		mosquittoConfig.Credentials.UserName = settings.LocalUsername
		mosquittoConfig.Credentials.Password = settings.LocalPassword
	}

	if err := SetupLocalTLS(mosquittoConfig, settings, logger); err != nil {
		return nil, err
	}

	return conn.NewMQTTConnection(mosquittoConfig, localClientID, logger)
}

// CreateCloudConnection creates a remote mosquitto connection.
func CreateCloudConnection(
	settings *LocalConnectionSettings,
	cleanSession bool,
	logger watermill.LoggerAdapter,
) (*conn.MQTTConnection, error) {
	mosquittoConfig, err := conn.NewMQTTClientConfig(settings.LocalAddress)
	if err != nil {
		return nil, err
	}

	mosquittoConfig.CleanSession = cleanSession
	mosquittoConfig.MaxReconnectInterval = 5 * time.Second
	mosquittoConfig.MinReconnectInterval = 5 * time.Second
	mosquittoConfig.ConnectRetryInterval = 5 * time.Second

	if len(settings.LocalUsername) > 0 {
		mosquittoConfig.Credentials.UserName = settings.LocalUsername
		mosquittoConfig.Credentials.Password = settings.LocalPassword
	}

	if err := SetupLocalTLS(mosquittoConfig, settings, logger); err != nil {
		return nil, err
	}

	return conn.NewMQTTConnection(mosquittoConfig, cloudClientID, logger)
}

// SetupTracing initializes the messages tracing.
func SetupTracing(router *message.Router, logger logger.Logger) {
	tracingPrefixes := os.Getenv("TRACE_TOPIC_PREFIX")
	if len(tracingPrefixes) > 0 {
		router.AddMiddleware(conn.NewTrace(logger, strings.Split(tracingPrefixes, ",")))
	}
}

// SetupLocalTLS creates a local ssl configuration.
func SetupLocalTLS(
	mosquittoConfig *conn.Configuration,
	settings *LocalConnectionSettings,
	logger watermill.LoggerAdapter,
) error {
	u, err := url.Parse(settings.LocalAddress)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "wss", "ssl", "tls", "mqtts", "mqtt+ssl", "tcps":
		tlsConfig, err := NewLocalTLSConfig(settings, logger)
		if err != nil {
			return err
		}
		mosquittoConfig.TLSConfig = tlsConfig
	default:
		// unsupported
	}

	return nil
}

// NewHonoSub returns subscriber for the Hono message connection.
func NewHonoSub(logger watermill.LoggerAdapter, honoClient *conn.MQTTConnection) message.Subscriber {
	return conn.NewBufferedSubscriber(honoClient, 256, false, logger, nil)
}

// NewHonoPub returns publisher for the Hono message connection.
func NewHonoPub(logger watermill.LoggerAdapter, honoClient *conn.MQTTConnection) message.Publisher {
	honoPub := conn.NewPublisher(honoClient, conn.QosAtLeastOnce, logger, nil)

	if rate, err := strconv.ParseInt(os.Getenv("MESSAGE_RATE"), 0, 64); err == nil {
		if rate <= 0 {
			return honoPub
		}
		return conn.NewRateLimiter(honoPub, rate, time.Second)
	}

	return honoPub
}

// NewOnlineHonoPub returns publisher for the Hono message connection in online mode.
func NewOnlineHonoPub(logger watermill.LoggerAdapter, honoClient *conn.MQTTConnection) message.Publisher {
	ackTimeout := 15 * time.Second
	if t, err := strconv.ParseInt(os.Getenv("HUB_PUBLISH_ACK_TIMEOUT"), 0, 64); err == nil {
		if t > 0 {
			ackTimeout = time.Duration(t) * time.Second
		}
	}
	return conn.NewOnlinePublisher(honoClient, conn.QosAtLeastOnce, ackTimeout, logger, nil)
}

// LocalConnect connects to the local client.
func LocalConnect(localClient *conn.MQTTConnection, logger watermill.LoggerAdapter) error {
	b := backoff.NewConstantBackOff(5 * time.Second)
	ticker := backoff.NewTicker(b)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	logMessage := true

	for range ticker.C {
		future := localClient.Connect()

		select {
		case <-future.Done():
			err := future.Error()
			if err == nil {
				return nil
			}

			if _, retry := routing.StatusCause(err); !retry {
				return err
			}

			if logMessage {
				logMessage = false
				logger.Error("Cannot connect to local broker", err, nil)
			}

		case <-sigs:
			return errors.New("connect to local broker cancelled by signal")
		}
	}

	return nil
}

// HonoConnect connects to the Hono client.
func HonoConnect(sigs <-chan os.Signal,
	statusPub message.Publisher,
	honoClient *conn.MQTTConnection,
	logger logger.Logger,
) error {
	b := honoClient.ConnectBackoff()
	b.Reset()

	logFields := watermill.LogFields{
		"mqtt_url": honoClient.URL(),
		"clientid": honoClient.ClientID(),
	}

	for {
		future := honoClient.Connect()

		select {
		case <-future.Done():
			if err := future.Error(); err != nil {
				logger.Error("Cannot connect to Hub", err, logFields)

				cause, retry := routing.StatusCause(err)
				routing.SendStatus(cause, statusPub, logger)
				if !retry {
					return err
				}

			} else {
				return nil
			}

		case <-sigs:
			return context.Canceled
		}

		waitTime := b.NextBackOff()
		if waitTime == backoff.Stop {
			logger.Error("Reconnect stopped", nil, logFields)
			return nil
		}

		logger.Debug(fmt.Sprintf("Reconnect after %v", waitTime.Round(time.Second)), logFields)

		select {
		case <-sigs:
			return context.Canceled

		case <-time.After(waitTime):
			//go on
		}
	}
}
