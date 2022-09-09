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

package testutil

import (
	"os"

	"github.com/eclipse-kanto/suite-connector/connector"
)

// NewLocalConfig returns the testing connector configuration.
func NewLocalConfig() (*connector.Configuration, error) {
	uri := os.Getenv("TEST_MQTT_URI")

	if len(uri) == 0 {
		uri = "tcp://localhost:1883"
	}

	cfg, err := connector.NewMQTTClientConfig(uri)
	if err != nil {
		return nil, err
	}

	cfg.ConnectRetryInterval = 0

	testUser := os.Getenv("TEST_MQTT_USER")
	if len(testUser) == 0 {
		testUser = "admin"
	}

	testPass := os.Getenv("TEST_MQTT_PASS")
	if len(testPass) == 0 {
		testPass = "admin"
	}

	cfg.Credentials.UserName = testUser
	cfg.Credentials.Password = testPass

	return cfg, nil
}
