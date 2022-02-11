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

package testutil

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
)

const (
	logFlags int = log.LstdFlags | log.Lmicroseconds | log.Lshortfile
)

func init() {
	if nop := os.Getenv("TEST_NOP_LOGGER"); len(nop) > 0 {
		log.SetOutput(io.Discard)
	} else {
		pahoLogLevel := os.Getenv("TEST_MQTT_LOG_LEVEL")
		if len(pahoLogLevel) > 0 {
			pahoLog := log.New(os.Stdout, fmt.Sprintf("[%s] ", "paho"), logFlags)
			logger.ConfigMQTT(logger.ParseLogLevel(pahoLogLevel), pahoLog)
		}
	}
}

// NewLogger returns system output logger.
func NewLogger(name string, level logger.LogLevel) logger.Logger {
	if nop := os.Getenv("TEST_NOP_LOGGER"); len(nop) > 0 {
		logout := log.New(io.Discard, fmt.Sprintf("[%s] ", name), logFlags)
		return logger.NewLogger(logout, level)
	}
	logout := log.New(os.Stdout, fmt.Sprintf("[%s] ", name), logFlags)
	return logger.NewLogger(logout, level)
}

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
