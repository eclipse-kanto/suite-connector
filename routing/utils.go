// Copyright (c) 2023 Contributors to the Eclipse Foundation
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
	"strings"

	"github.com/pkg/errors"
)

// command topic is in format command/tenantID/gatewayID/deviceID/suffix
func fixCommand(topic string) (string, error) {
	segments := strings.Split(topic, "/")

	if len(segments) < 5 {
		return "", errors.Errorf("Invalid command topic %s", topic)
	}

	var buff strings.Builder
	buff.Grow(256)

	buff.WriteString("command//")

	//check if the deviceID is not empty
	if len(segments[3]) > 0 {
		buff.WriteString(segments[2])
		buff.WriteString(":")
		buff.WriteString(segments[3])
	}

	for i := 4; i < len(segments); i++ {
		buff.WriteString("/")
		buff.WriteString(segments[i])
	}

	return buff.String(), nil
}

// deviceID is in format gatewayID:ID
func stripGatewayID(gatewayID, deviceID string) (string, error) {
	if strings.HasPrefix(deviceID, gatewayID) {

		if len(deviceID) <= len(gatewayID)+1 {
			return "", nil
		}

		return deviceID[len(gatewayID)+1:], nil
	}

	return "", errors.Errorf("Invalid deviceID %s for gateway %s", deviceID, gatewayID)
}
