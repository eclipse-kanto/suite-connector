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

package connector

import (
	"crypto/tls"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

const (
	keepAliveTimeout = 20 * time.Second

	connectTimeout = 30 * time.Second

	maxReconnectInterval = time.Minute

	connectRetryInterval = time.Minute

	disconnectQuiesce = 10000

	subTimeout   = 15 * time.Second
	unsubTimeout = 5 * time.Second
)

// Credentials defines the connection user name and password.
type Credentials struct {
	UserName string
	Password string
}

// Configuration defines the MQTT connection system data.
type Configuration struct {
	URL string

	CleanSession bool
	NoOpStore    bool

	ConnectTimeout       time.Duration
	KeepAliveTimeout     time.Duration
	ConnectRetryInterval time.Duration

	ExternalReconnect    bool
	BackoffMultiplier    float64
	MaxReconnectInterval time.Duration
	MinReconnectInterval time.Duration

	Credentials Credentials

	WillTopic   string
	WillMessage []byte
	WillQos     Qos
	WillRetain  bool

	TLSConfig *tls.Config
}

// Validate validates the configuration data.
func (c *Configuration) Validate() error {
	if len(c.URL) == 0 {
		return errors.New("MQTT url is missing")
	}

	_, err := url.Parse(c.URL)
	if err != nil {
		return errors.Wrap(err, "invalid MQTT url")
	}

	if c.MinReconnectInterval < 0 {
		return errors.New("MinReconnectInterval < 0")
	}

	if c.MaxReconnectInterval < 0 {
		return errors.New("MaxReconnectInterval < 0")
	}

	if c.MinReconnectInterval > c.MaxReconnectInterval {
		return errors.New("MinReconnectInterval > MaxReconnectInterval")
	}

	if c.BackoffMultiplier < 1 {
		return errors.New("BackoffMultiplier < 1")
	}

	return nil
}

// NewMQTTClientConfig returns the configuration a broker.
func NewMQTTClientConfig(broker string) (*Configuration, error) {
	u, err := url.ParseRequestURI(broker)
	if err != nil {
		return nil, err
	}

	return &Configuration{
		URL:                  u.String(),
		ConnectTimeout:       connectTimeout,
		KeepAliveTimeout:     keepAliveTimeout,
		ConnectRetryInterval: connectRetryInterval,
		MinReconnectInterval: maxReconnectInterval,
		MaxReconnectInterval: maxReconnectInterval,
		BackoffMultiplier:    2,
	}, nil
}
