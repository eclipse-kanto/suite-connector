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

package config

import (
	"encoding/base64"
	"fmt"

	parser "github.com/Jeffail/gabs/v2"
	"github.com/pkg/errors"
)

// The json paths to the gateway connection settings filled in from provisioning.json
const (
	deviceIDPath        = "hub.credentials.deviceId"
	tenantIDPath        = "hub.credentials.tenantId"
	policyIDPath        = "things.thing.policyId"
	authIDPath          = "hub.credentials.authId"
	adaptersPath        = "hub.credentials.adapters"
	credentialsTypePath = "hub.credentials.type"
	certificate         = "x509-cert"
	secretsPath         = "hub.credentials.secrets"
	password            = "password"
	passwordBase64      = "passwordBase64"
	authoritiesPath     = "hub.device.authorities"
	provisioningEnabled = "auto-provisioning-enabled"

	defaultStringValue = ""
)

// ParseProvisioningJSON parse and get settings values using the provided json content.
// The expected format should match the format used in the provisioning.json file.
// All values that are not present or have unexpected format are not filled in.
// Returns error if the provided data is not a valid json format.
func ParseProvisioningJSON(data []byte, settings *HubConnectionSettings) error {
	jsonData, err := parser.ParseJSON(data)
	if err != nil {
		return errors.New("provided data is not a valid json")
	}

	readValue(&settings.DeviceID, jsonData, deviceIDPath)
	readValue(&settings.TenantID, jsonData, tenantIDPath)
	readValue(&settings.PolicyID, jsonData, policyIDPath)
	readValue(&settings.AuthID, jsonData, authIDPath)
	readAddress(&settings.Address, jsonData, adaptersPath)
	readPassword(&settings.Password, jsonData, secretsPath)

	settings.UseCertificate = getString(jsonData, credentialsTypePath) == certificate
	settings.AutoProvisioningEnabled = hasElement(jsonData, authoritiesPath, provisioningEnabled)

	return nil
}

func readPassword(field *string, jsonData *parser.Container, path string) {
	for _, secret := range jsonData.Search(parser.DotPathToSlice(path)...).Children() {
		for key, value := range secret.ChildrenMap() {

			if key == passwordBase64 {
				if passwordStr, err := base64.StdEncoding.DecodeString(value.Data().(string)); err == nil {
					*field = string(passwordStr)
					return
				}
			}

			if key == password {
				if passwordStr, ok := value.Data().(string); ok {
					*field = passwordStr
					return
				}
			}
		}
	}
}

func hasElement(jsonData *parser.Container, path string, element string) bool {
	for _, child := range jsonData.Search(parser.DotPathToSlice(path)...).Children() {
		if child.Data() == element {
			return true
		}
	}
	return false
}

func readValue(field *string, jsonData *parser.Container, path string) {
	if value, ok := jsonData.Path(path).Data().(string); ok {
		*field = value
	}
}

func getString(jsonData *parser.Container, path string) string {
	if value, ok := jsonData.Path(path).Data().(string); ok {
		return value
	}
	return defaultStringValue
}

func readAddress(field *string, jsonData *parser.Container, path string) {
	for _, adaptersContainer := range jsonData.Search(parser.DotPathToSlice(path)...).Children() {
		for key, value := range adaptersContainer.ChildrenMap() {
			if key == "type" {
				if str, ok := value.Data().(string); ok && str == "mqtt" {
					*field = mqttURL(adaptersContainer)
				}
			}
		}
	}
}

func mqttURL(mqttSettings *parser.Container) string {
	uri := getString(mqttSettings, "uri")
	if uri != defaultStringValue {
		if port, ok := mqttSettings.Search("port").Data().(float64); ok {
			return fmt.Sprintf("%s:%.0f", uri, port)
		}
	}
	return uri
}
