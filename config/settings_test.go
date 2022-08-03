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
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/util"
	"go.uber.org/goleak"

	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eclipse-kanto/suite-connector/testutil"
)

const (
	testProvisioning = "testdata/provisioning.json"

	testConfig        = "testdata/config.json"
	testDynamicConfig = "testdata/dynamicConfig.json"
)

type TestSettings struct {
	// The difference from the original Settings struct is the 'omitempty' option that is added
	// to assist dynamic configuration file creation, see createConfig func.
	ProvisioningFile string `json:"provisioningFile,omitempty"`

	DeviceID        string `json:"deviceId,omitempty"`
	DeviceIDPattern string `json:"deviceIdPattern,omitempty"`
	TenantID        string `json:"tenantId,omitempty"`
	PolicyID        string `json:"policyId,omitempty"`
	Address         string `json:"address,omitempty"`
	Password        string `json:"password,omitempty"`
	ClientID        string `json:"clientId,omitempty"`

	LocalAddress  string `json:"localAddress,omitempty"`
	LocalUsername string `json:"localUsername,omitempty"`
	LocalPassword string `json:"localPassword,omitempty"`

	AuthID string `json:"authId,omitempty"`

	CACert    string `json:"cacert,omitempty"`
	Cert      string `json:"cert,omitempty"`
	Key       string `json:"key,omitempty"`
	TPMKey    string `json:"tpmKey,omitempty"`
	TPMKeyPub string `json:"tpmKeyPub,omitempty"`
	TPMHandle uint64 `json:"tpmHandle,omitempty"`
	TPMDevice string `json:"tpmDevice,omitempty"`

	LogFile       string          `json:"logFile,omitempty"`
	LogLevel      logger.LogLevel `json:"logLevel,omitempty"`
	LogFileSize   int             `json:"logFileSize,omitempty"`
	LogFileCount  int             `json:"logFileCount,omitempty"`
	LogFileMaxAge int             `json:"logFileMaxAge,omitempty"`
}

func TestDeviceIdPatternDependencies(t *testing.T) {
	settings := DefaultSettings()
	settings.DeviceIDPattern = "namespace123:{{subject-dn}}"
	settings.CACert = "testdata/certificate.pem"
	settings.Cert = "testdata/certificate.pem"

	require.NoError(t, settings.ValidateStatic())

	settings.DeviceID = "deviceId"
	assert.Error(t, settings.ValidateStatic())

	// missing certificate files
	settings.Cert = ""
	assert.Error(t, settings.ValidateStatic())

	settings.DeviceID = ""
	assert.Error(t, settings.ValidateStatic())
}

func TestConfigEmpty(t *testing.T) {
	configPath := "configEmpty.json"

	err := ioutil.WriteFile(configPath, []byte{}, os.ModePerm)
	require.NoError(t, err)
	defer func() {
		os.Remove(configPath)
	}()
	require.True(t, util.FileExists(configPath))

	cmd := new(Settings)
	assert.NoError(t, ReadConfig(configPath, cmd))
}

func TestMergeError(t *testing.T) {
	defer goleak.VerifyNone(t)

	cmd := make(map[string]interface{})
	cmd["DeviceID"] = 17
	doTestErrorStatus(t, testProvisioning, routing.StatusProvisioningError, cmd)
}

func TestApplySettingsValidateError(t *testing.T) {
	defer goleak.VerifyNone(t)

	cmd := make(map[string]interface{})
	cmd["DeviceID"] = ""
	doTestErrorStatus(t, testProvisioning, routing.StatusProvisioningError, cmd)
}

func TestProvisioningUpdate(t *testing.T) {
	defer goleak.VerifyNone(t)

	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	statusPub := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(4),
		},
		logger,
	)
	defer statusPub.Close()

	sMsg, err := statusPub.Subscribe(context.Background(), routing.TopicConnectionStatus)
	require.NoError(t, err)

	settings := DefaultSettings()
	settings.DeviceID = "dummy:Dummy"
	settings.TenantID = "Dummy"
	settings.LocalAddress = "invalid"
	cmd := make(map[string]interface{})

	require.NoError(t, ApplySettings(true, settings, cmd, statusPub, logger))

	receivedMsgs, ok := subscriber.BulkRead(sMsg, 1, time.Second*3)
	require.True(t, ok)

	status := new(routing.ConnectionStatus)
	err = json.Unmarshal(receivedMsgs[0].Payload, status)
	require.NoError(t, err)

	assert.False(t, status.Connected)
	assert.Equal(t, routing.StatusProvisioningUpdate, status.Cause)
}

func TestConfigInvalid(t *testing.T) {
	settings := DefaultSettings()
	assert.Error(t, ReadConfig("settings_test.go", settings))
	assert.Error(t, ReadConfig("settings_test.go", nil))

	settings.LogFileCount = 0
	assert.Error(t, settings.ValidateStatic())

	settings.LogFileCount = 1
	settings.LocalAddress = ""
	assert.Error(t, settings.ValidateStatic())
}

func TestConfig(t *testing.T) {
	expSettings := DefaultSettings()
	expSettings.DeviceID = "config:deviceId"
	expSettings.PolicyID = "policyId_config"
	expSettings.LocalUsername = "localUsername_config"
	expSettings.LocalPassword = "localPassword_config"
	expSettings.CACert = ""
	expSettings.LogFile = "logFile_config"
	expSettings.LogLevel = logger.DEBUG

	settings := DefaultSettings()
	require.NoError(t, ReadConfig(testConfig, settings))
	assert.Equal(t, expSettings, settings)

	settings.TenantID = "tenantId_config"
	assert.NoError(t, settings.ValidateStatic())
	assert.NoError(t, settings.ValidateDynamic())
}

func TestDefaults(t *testing.T) {
	settings := DefaultSettings()
	assert.NoError(t, settings.ValidateStatic())

	settings.CACert = "testdata/certificate.pem"
	assert.NoError(t, settings.ValidateStatic())

	assert.NotNil(t, settings.LocalConnection())
	assert.NotNil(t, settings.HubConnection())
	assert.Equal(t, settings.ProvisioningFile, settings.Provisioning())
	assert.Equal(t, settings, settings.DeepCopy())
}

func TestProvisioningMissing(t *testing.T) {
	defer goleak.VerifyNone(t)

	cmd := make(map[string]interface{})
	doTestErrorStatus(t, "non-existing.json", routing.StatusProvisioningMissing, cmd)

	settings := DefaultSettings()
	require.NoError(t, ReadProvisioning("non-existing.json", &settings.HubConnectionSettings))
}

func TestProvisioningInvalid(t *testing.T) {
	defer goleak.VerifyNone(t)

	expSettings := DefaultSettings()

	settings := DefaultSettings()
	require.Error(t, ReadProvisioning("settings_test.go", &settings.HubConnectionSettings))

	assert.Equal(t, expSettings, settings)

	cmd := make(map[string]interface{})
	doTestErrorStatus(t, "settings_test.go", routing.StatusProvisioningError, cmd)
}

func TestProvisioning(t *testing.T) {
	expSettings := DefaultSettings()
	expSettings.CACert = "testdata/certificate.pem"
	expSettings.DeviceID = "test:settings"
	expSettings.TenantID = "test:settings_hub"
	expSettings.PolicyID = "test:settings2"
	expSettings.Address = "mqtts://mqtt.example.com:8888"
	expSettings.Password = "test"
	expSettings.AuthID = "test:settings_auth"
	expSettings.AutoProvisioningEnabled = false

	settings := DefaultSettings()
	settings.CACert = "testdata/certificate.pem"
	require.NoError(t, ReadProvisioning(testProvisioning, &settings.HubConnectionSettings))

	assert.Equal(t, expSettings, settings)

	assert.NoError(t, settings.ValidateDynamic())
	assert.NoError(t, settings.ValidateStatic())
}

func TestValidStructPointer(t *testing.T) {
	assert.False(t, validStructPointer(nil))

	m := make(map[string]interface{})
	assert.False(t, validStructPointer(m))
	assert.False(t, validStructPointer(&m))

	var s Settings
	assert.False(t, validStructPointer(s))
	assert.True(t, validStructPointer(&s))
}

func TestConfigWithProvisioningMissing(t *testing.T) {
	s := &TestSettings{
		ProvisioningFile: "provisioning_config",
		DeviceID:         "config:deviceId",
		PolicyID:         "policyId_config",
		LocalUsername:    "localUsername_config",
		LocalPassword:    "localPassword_config",
		TenantID:         "tenantId_config",
		CACert:           "testdata/certificate.pem",
		LogLevel:         logger.DEBUG,
	}

	content, err := json.MarshalIndent(&s, "", "\t")
	require.NoError(t, err)

	err = ioutil.WriteFile(testDynamicConfig, []byte(content), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testDynamicConfig)

	settings := DefaultSettings()
	require.NoError(t, ReadConfig(testDynamicConfig, settings))

	assert.NoError(t, settings.ValidateDynamic())
	assert.NoError(t, settings.ValidateStatic())
}

func TestProvisioningWithDeviceIDPatternConflict(t *testing.T) {
	expSettings := DefaultSettings()
	expSettings.ProvisioningFile = testProvisioning
	expSettings.CACert = "testdata/certificate.pem"

	expSettings.DeviceID = "test:settings"
	expSettings.DeviceIDPattern = "namespace123:{{subject-dn}}"
	expSettings.Cert = "cert.pem"
	expSettings.TenantID = "test:settings_hub"
	expSettings.PolicyID = "test:settings2"
	expSettings.Address = "mqtts://mqtt.example.com:8888"
	expSettings.Password = "test"
	expSettings.AuthID = "test:settings_auth"
	expSettings.AutoProvisioningEnabled = false

	require.Error(t, expSettings.ValidateStatic())

	settings := DefaultSettings()
	settings.Cert = "cert.pem"
	settings.CACert = "testdata/certificate.pem"
	settings.ProvisioningFile = testProvisioning
	settings.DeviceIDPattern = "namespace123:{{subject-dn}}"
	require.NoError(t, ReadProvisioning(testProvisioning, &settings.HubConnectionSettings))

	assert.Equal(t, expSettings, settings)
	assert.NoError(t, settings.ValidateDynamic())

	settings = DefaultSettings()
	settings.DeviceID = "error_test"
	assert.Error(t, settings.ValidateDynamic())
}

func doTestErrorStatus(t *testing.T, provJSON, expected string, cmd map[string]interface{}) {
	logger := testutil.NewLogger("testing", logger.DEBUG, t)

	statusPub := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(4),
		},
		logger,
	)
	defer statusPub.Close()

	sMsg, err := statusPub.Subscribe(context.Background(), routing.TopicConnectionStatus)
	require.NoError(t, err)

	settings := DefaultSettings()
	settings.ProvisioningFile = provJSON

	require.Error(t, ApplySettings(false, settings, cmd, statusPub, logger))

	receivedMsgs, ok := subscriber.BulkRead(sMsg, 1, time.Second*3)
	require.True(t, ok)

	status := new(routing.ConnectionStatus)
	err = json.Unmarshal(receivedMsgs[0].Payload, status)
	require.NoError(t, err)

	assert.False(t, status.Connected)
	assert.Equal(t, expected, status.Cause)
}
