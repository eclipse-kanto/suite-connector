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

package config_test

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/testutil"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

func TestCreateHonoClientNoCacert(t *testing.T) {
	defer goleak.VerifyNone(t)

	hubSettings := config.HubConnectionSettings{
		DeviceID: "org.eclipse.kanto:test",
		TenantID: "t6ea0f08c_hub",
		AuthID:   "org.eclipse.kanto_test",
		Address:  "mqtts://mqtt.example.com:8883",
	}
	settings := &config.Settings{
		HubConnectionSettings: hubSettings,
	}
	require.NoError(t, settings.ValidateDynamic())
	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	honoClient, _, err := config.CreateHubConnection(&settings.HubConnectionSettings, false, logger)
	require.Error(t, err)
	assert.Nil(t, honoClient)
}

func TestCreateHonoClient(t *testing.T) {
	defer goleak.VerifyNone(t)

	hubSettings := config.HubConnectionSettings{
		DeviceID: "org.eclipse.kanto:test",
		TenantID: "t6ea0f08c_hub",
		AuthID:   "org.eclipse.kanto_test",
		Address:  "mqtts://mqtt.example.com:8883",
		TLSSettings: config.TLSSettings{
			CACert: "testdata/certificate.pem",
		},
	}
	settings := &config.Settings{
		HubConnectionSettings: hubSettings,
	}
	require.NoError(t, settings.ValidateDynamic())
	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	t.Setenv("HUB_CONNECT_INIT", "60")
	t.Setenv("HUB_CONNECT_MAX", "120")
	t.Setenv("HUB_CONNECT_MUL", "2.0")

	honoClient, cleanup, err := config.CreateHubConnection(&settings.HubConnectionSettings, false, logger)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NotNil(t, honoClient)
	defer cleanup()

	hubSettings = config.HubConnectionSettings{
		DeviceID:        "org.eclipse.kanto:test",
		TenantID:        "t6ea0f08c_hub",
		Address:         "mqtts://mqtt.example.com:8883",
		DeviceIDPattern: "namespace123:{{subject-dn}}",
		TLSSettings: config.TLSSettings{
			CACert: "testdata/certificate.pem",
			Cert:   "testdata/certificate.pem",
			Key:    "testdata/key.pem",
		},
	}

	honoClient, cleanup, err = config.CreateHubConnection(&hubSettings, false, logger)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NotNil(t, honoClient)
	defer cleanup()
}

func TestCreateHonoClientInvalidURL(t *testing.T) {
	defer goleak.VerifyNone(t)

	hubSettings := &config.HubConnectionSettings{
		Address: "invalid#%",
	}

	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	honoClient, _, err := config.CreateHubConnection(hubSettings, false, logger)
	require.Error(t, err)
	assert.Nil(t, honoClient)
}

func TestCreateLocalConnectionValidURL(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := &config.LocalConnectionSettings{
		LocalAddress:  "tcp://localhost:1883",
		LocalUsername: "test",
	}
	assert.NotNil(t, settings.LocalAddress) // assert default is set
	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	localConnection, err := config.CreateLocalConnection(settings, logger)
	require.NoError(t, err)
	assert.NotNil(t, localConnection)
}

func TestCreateLocalConnectionInvalidURL(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := &config.LocalConnectionSettings{
		LocalAddress: "invalid#%",
	}
	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	cloudClient, err := config.CreateLocalConnection(settings, logger)
	require.Error(t, err)
	assert.Nil(t, cloudClient)
}

func TestCreateCloudConnectionValidURL(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := &config.LocalConnectionSettings{
		LocalAddress:  "tcp://localhost:1883",
		LocalUsername: "test",
	}
	assert.NotNil(t, settings.LocalAddress)
	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	cloudClient, err := config.CreateCloudConnection(settings, false, logger)
	require.NoError(t, err)
	assert.NotNil(t, cloudClient)
}

func TestCreateCloudConnectionInvalidURL(t *testing.T) {
	defer goleak.VerifyNone(t)

	settings := &config.LocalConnectionSettings{
		LocalAddress: "invalid#%",
	}
	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	cloudClient, err := config.CreateCloudConnection(settings, false, logger)
	require.Error(t, err)
	assert.Nil(t, cloudClient)
}

func TestNewRouterEmptyConfig(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	router, err := message.NewRouter(message.RouterConfig{}, logger)
	require.NoError(t, err)
	assert.NotNil(t, router)
	router.Close()
}

func TestSetupTracing(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)
	router, _ := message.NewRouter(message.RouterConfig{}, logger)
	assert.NotNil(t, router)
	defer router.Close()

	t.Setenv("TRACE_TOPIC_PREFIX", "c/")

	config.SetupTracing(router, logger)
}

func TestHonoPub(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	client, err := conn.NewMQTTConnection(cfg, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	pub := config.NewHonoPub(logger, client)
	require.NotNil(t, pub)
	assert.NoError(t, pub.Close())

	t.Setenv("MESSAGE_RATE", "25")

	pub = config.NewHonoPub(logger, client)
	require.NotNil(t, pub)
	assert.NoError(t, pub.Close())

	t.Setenv("MESSAGE_RATE", "-1")

	pub = config.NewHonoPub(logger, client)
	require.NotNil(t, pub)
	assert.NoError(t, pub.Close())
}

func TestOnlineHonoPub(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	client, err := conn.NewMQTTConnection(cfg, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	pub := config.NewOnlineHonoPub(logger, client)
	require.NotNil(t, pub)
	assert.NoError(t, pub.Close())

	t.Setenv("HUB_PUBLISH_ACK_TIMEOUT", "10")

	pub = config.NewOnlineHonoPub(logger, client)
	require.NotNil(t, pub)
	assert.NoError(t, pub.Close())
}

func TestHonoSub(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg, err := testutil.NewLocalConfig()
	require.NoError(t, err)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	client, err := conn.NewMQTTConnection(cfg, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	sub := config.NewHonoSub(logger, client)
	require.NotNil(t, sub)
	assert.NoError(t, sub.Close())
}
