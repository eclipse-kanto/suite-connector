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
	"encoding/json"
	"io/ioutil"
	"net/url"
	"reflect"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/util"
)

// SettingsAccessor is used accessing embedded struct members.
type SettingsAccessor interface {
	// Provisioning returns the ProvisioningFile embedded field.
	Provisioning() string

	// LocalConnection returns the LocalConnectionSettings embedded field.
	LocalConnection() *LocalConnectionSettings

	// HubConnection returns HubConnectionSettings embedded field.
	HubConnection() *HubConnectionSettings

	// ValidateDynamic validates settings instance
	ValidateDynamic() error

	// Creates deep copy of this instance.
	DeepCopy() SettingsAccessor
}

// Settings defines all configurable data that is used to setup the Suite Connector.
type Settings struct {
	logger.LogSettings

	HubConnectionSettings

	LocalConnectionSettings

	ProvisioningFile string `json:"provisioningFile"`
}

// ValidateDynamic validates the settings that are changed runtime.
func (settings *Settings) ValidateDynamic() error {
	return settings.HubConnectionSettings.Validate()
}

// Provisioning implementation.
func (settings *Settings) Provisioning() string {
	return settings.ProvisioningFile
}

// LocalConnection implementation.
func (settings *Settings) LocalConnection() *LocalConnectionSettings {
	return &settings.LocalConnectionSettings
}

// HubConnection implementation.
func (settings *Settings) HubConnection() *HubConnectionSettings {
	return &settings.HubConnectionSettings
}

// DeepCopy implementation.
func (settings *Settings) DeepCopy() SettingsAccessor {
	clone := *settings
	return &clone
}

// ValidateStatic validates the settings that could not be changed runtime.
func (settings *Settings) ValidateStatic() error {
	if err := settings.LogSettings.Validate(); err != nil {
		return err
	}

	if err := settings.LocalConnectionSettings.Validate(); err != nil {
		return err
	}

	u, err := url.ParseRequestURI(settings.Address)
	if err != nil {
		return err
	}
	if isConnectionSecure(u.Scheme) {
		if len(settings.CACert) > 0 {
			if !util.FileExists(settings.CACert) {
				return errors.Errorf("failed to read CA certificate file '%s'", settings.CACert)
			}
		} else {
			return errors.New("cannot use secure connection when the CA certificate file is not provided")
		}
	}

	if len(settings.DeviceIDPattern) > 0 {
		if len(settings.DeviceID) > 0 {
			return errors.Errorf(
				"cannot use -deviceIdPattern flag value '%s' when -deviceId flag value '%s' is also provided",
				settings.DeviceIDPattern, settings.DeviceID,
			)
		}

		if len(settings.Cert) == 0 {
			return errors.Errorf(
				"cannot use -deviceIdPattern flag value '%s' when the certificate file is not provided",
				settings.DeviceIDPattern,
			)
		}
	}

	return nil
}

// DefaultSettings returns the default settings.
func DefaultSettings() *Settings {
	return &Settings{
		HubConnectionSettings: HubConnectionSettings{
			Address:                 "mqtts://mqtt.bosch-iot-hub.com:8883",
			AutoProvisioningEnabled: true,
			TLSSettings: TLSSettings{
				CACert: "iothub.crt",
			},
		},
		LocalConnectionSettings: LocalConnectionSettings{
			LocalAddress: "tcp://localhost:1883",
		},
		LogSettings: logger.LogSettings{
			LogFile:       "log/suite-connector.log",
			LogLevel:      logger.INFO,
			LogFileSize:   2,
			LogFileCount:  5,
			LogFileMaxAge: 28,
		},
		ProvisioningFile: "provisioning.json",
	}
}

// ReadConfig reads the configuration file provided settings.
func ReadConfig(path string, config interface{}) error {
	if !validStructPointer(config) {
		return errors.New("config not a pointer to struct")
	}

	if util.FileExists(path) {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "error reading configuration file: %s", path)
		}

		if len(data) == 0 {
			return nil
		}

		if err = json.Unmarshal(data, config); err != nil {
			return errors.Wrapf(err, "provided configuration file '%s' is not a valid JSON", path)
		}
	}

	return nil
}

// ReadProvisioning reads the provisioning file provided settings.
func ReadProvisioning(path string, settings *HubConnectionSettings) error {
	if util.FileExists(path) {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "error reading provisioning file: %v", path)
		}

		if err := ParseProvisioningJSON(data, settings); err != nil {
			return errors.Wrapf(err, "error reading provisioning file: %v", path)
		}
	}
	return nil
}

// ApplySettings applies all launch settings.
func ApplySettings(
	cleanSession bool,
	settings SettingsAccessor,
	args map[string]interface{},
	statusPub message.Publisher,
	logger logger.Logger,
) error {
	path := settings.Provisioning()

	if !util.FileExists(path) {
		if err := settings.ValidateDynamic(); err != nil {
			routing.SendStatus(routing.StatusProvisioningMissing, statusPub, logger)
			return err
		}
	} else {
		if err := ReadProvisioning(path, settings.HubConnection()); err != nil {
			routing.SendStatus(routing.StatusProvisioningError, statusPub, logger)
			return err
		}

		if err := mergo.Map(settings, args, mergo.WithOverwriteWithEmptyValue); err != nil {
			routing.SendStatus(routing.StatusProvisioningError, statusPub, logger)
			return err
		}

		if err := settings.ValidateDynamic(); err != nil {
			routing.SendStatus(routing.StatusProvisioningError, statusPub, logger)
			return err
		}

		logger.Infof("Starting with '%s' and '%s':%v",
			path, provisioningEnabled, settings.HubConnection().AutoProvisioningEnabled)
	}

	if cleanSession {
		routing.SendStatus(routing.StatusProvisioningUpdate, statusPub, logger)
	}

	return nil
}

func validStructPointer(v interface{}) bool {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return false
	}

	rv = rv.Elem()

	return rv.Kind() == reflect.Struct
}
