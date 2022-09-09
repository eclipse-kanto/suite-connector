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

package connector

import (
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/eclipse-kanto/suite-connector/logger"
)

func isDebugEnabled(l watermill.LoggerAdapter) bool {
	if log, ok := l.(logger.Logger); ok {
		return log.IsDebugEnabled()
	}
	return true
}

func parseSubs(topic string) []string {
	return strings.Split(topic, ",")
}

func subFilters(qos Qos, subs []string) map[string]byte {
	filters := make(map[string]byte)
	for _, topic := range subs {
		filters[topic] = byte(qos)
	}
	return filters
}

func calcTimeout(keepAlive time.Duration) time.Duration {
	return time.Duration(float64(keepAlive) * 0.8)
}
