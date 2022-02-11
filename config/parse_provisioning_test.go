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

package config_test

import (
	"testing"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsingCorrectValuesProvisioning(t *testing.T) {
	testProv := `{
		"id": "namespace.for.test:octopusSuiteEdition110",
		"hub": {
			"credentials": {
				"tenantId": "tcdc240281d0e4b1eb66587b27c94fdf5_hub",
				"type": "hashed-password",
				"enabled": true,
				"secrets": [
					{
						"passwordBase64": "dGVzdA=="
					}
				],
				"deviceId": "namespace.for.test:octopusSuiteEdition110",
				"authId": "namespace.for.test_octopusSuiteEdition110",
				"adapters": [
					{
						"type": "http",
						"uri": "https://http.example.com",
						"host": "http.example.com",
						"port": 443
					},
					{
						"type": "mqtt",
						"uri": "mqtts://mqtt.example.com",
						"host": "mqtt.example.com",
						"port": 8883
					},
					{
						"type": "amqp",
						"uri": "amqps://amqp.example.com",
						"host": "amqp.example.com",
						"port": 5671
					}
				]
			},
			"device": {
				"enabled": true,
				"authorities": [
					"auto-provisioning-enabled"
				],
				"deviceId": "namespace.for.test:octopusSuiteEdition110"
			}
		},
		"things": {
			"thing": {
				"definition": "namespace.for.test.octopussuiteedition:OctopusSuiteEdition:1.1.0",
				"attributes": {},
				"features": {},
				"thingId": "namespace.for.test:octopusSuiteEdition110",
				"policyId": "namespace.for.test:octopusSuiteEdition110",
				"_policy": {}
			}
		}
	}`

	settings := &config.Settings{}
	err := config.ParseProvisioningJSON([]byte(testProv), &settings.HubConnectionSettings)
	require.NoError(t, err)
	err = settings.ValidateDynamic()
	require.NoError(t, err)

	assert.EqualValues(t, "tcdc240281d0e4b1eb66587b27c94fdf5_hub", settings.TenantID)
	assert.EqualValues(t, "namespace.for.test_octopusSuiteEdition110", settings.AuthID)
	assert.EqualValues(t, "namespace.for.test:octopusSuiteEdition110", settings.DeviceID)
	assert.EqualValues(t, "test", settings.Password)
	assert.EqualValues(t, true, settings.AutoProvisioningEnabled)
	assert.EqualValues(t, "namespace.for.test:octopusSuiteEdition110", settings.PolicyID)
	assert.EqualValues(t, false, settings.UseCertificate)
	assert.EqualValues(t, "mqtts://mqtt.example.com:8883", settings.Address)
}

func TestParsingPlainPasswordProvisioning(t *testing.T) {
	testProv := `{
		"id": "namespace.for.test:test",
		"hub": {
			"credentials": {
				"type": "hashed-password",
				"enabled": true,
				"secrets": [
					{
						"password": "dGVzdA=="
					}
				],
				"deviceId": "namespace.for.test:test"
			}
		}
	}`

	settings := &config.Settings{}
	err := config.ParseProvisioningJSON([]byte(testProv), &settings.HubConnectionSettings)
	require.NoError(t, err)
	require.NoError(t, err)

	assert.EqualValues(t, "namespace.for.test:test", settings.DeviceID)
	assert.EqualValues(t, "dGVzdA==", settings.Password)
	assert.EqualValues(t, false, settings.UseCertificate)
}

func TestParsingProvisioningEmptyKeys(t *testing.T) {
	testProv := `{
		"id": "",
		"hub": {
			"credentials": {
				"tenantId": "",
				"type": "hashed-password",
				"enabled": true,
				"secrets": [
					{
						"passwordBase64": ""
					}
				],
				"deviceId": "",
				"authId": "",
				"adapters": []
			},
			"device": {
				"enabled": true,
				"deviceId": ""
			}
		},
		"things": {
			"thing": {
				"definition": "",
				"attributes": {},
				"features": {},
				"thingId": "",
				"policyId": "",
				"_policy": {}
			}
		}
	}`
	parseAndAssertChanged(t, []byte(testProv), &config.Settings{})
}

func parseAndAssertChanged(t *testing.T, data []byte, exp *config.Settings) {
	settings := *exp
	error := config.ParseProvisioningJSON(data, &settings.HubConnectionSettings)
	assert.NoError(t, error)
	assert.EqualValues(t, exp, &settings)
}

func TestParsingProvisioningEmptyFile(t *testing.T) {
	testProv := `{
	}`
	parseAndAssertChanged(t, []byte(testProv), &config.Settings{})

	hubConSettings := config.HubConnectionSettings{
		DeviceID: "DeviceID",
		TenantID: "TenantID",
		Address:  "Address",
		AuthID:   "AuthID",
		Password: "Password",
		PolicyID: "PolicyID",
	}

	parseAndAssertChanged(t, []byte(testProv), &config.Settings{
		HubConnectionSettings: hubConSettings,
	})
}

func TestInvalidProvisioningFile(t *testing.T) {
	testProv := `}`
	settings := &config.Settings{}
	err := config.ParseProvisioningJSON([]byte(testProv), &settings.HubConnectionSettings)

	assert.EqualError(t, err, "provided data is not a valid json")
}

func TestInvalidValueTypes(t *testing.T) {
	testProv := `{
		"id": "namespace.for.test:octopusSuiteEdition110",
		"hub": {
			"credentials": {
				"tenantId": [ {"key": "tcdc240281d0e4b1eb66587b27c94fdf5_hub"}],
				"type": [ {"key": "hashed-password"}],
				"enabled": true,
				"secrets": [{"passwordBase643": "dGVzdA=="}],
				"deviceId": [ {"key": "namespace.for.test:octopusSuiteEdition110"}],
				"authId": [ {"key": "namespace.for.test_octopusSuiteEdition110"}],
				"adapters": [
					{
						"type": "mqtt",
						"uri": [{"key": "mqtts://mqtt.example.com"}],
						"host": "mqtt.example.com",
						"port": 8883
					}
				]
			},
			"device": {
				"enabled": true,
				"deviceId": "namespace.for.test:octopusSuiteEdition110"
			}
		},
		"things": {
			"thing": {
				"definition": "namespace.for.test.octopussuiteedition:OctopusSuiteEdition:1.1.0",
				"attributes": {},
				"features": {},
				"thingId": "namespace.for.test:octopusSuiteEdition110",
				"policyId": {"key": "namespace.for.test:octopusSuiteEdition110"},
				"_policy": {}
			}
		}
	}`
	parseAndAssertChanged(t, []byte(testProv), &config.Settings{})
}

func TestValidate(t *testing.T) {
	settings := config.Settings{}

	// DeviceID is missing
	error := settings.ValidateDynamic()
	assert.EqualError(t, error, "device ID or pattern is missing")
	settings.DeviceID = "namespace.for.test:octopusSuiteEdition110"

	// TenantID is missing
	error = settings.ValidateDynamic()
	assert.EqualError(t, error, "tenant ID is missing")
	settings.TenantID = "tcdc240281d0e4b1eb66587b27c94fdf5_hub"

	// RemoteMQTTURL is missing
	error = settings.ValidateDynamic()
	assert.EqualError(t, error, "remote broker address is missing")
	settings.Address = "mqtts://mqtt.example:8883"

	// No Error
	error = settings.ValidateDynamic()
	require.NoError(t, error)
}
