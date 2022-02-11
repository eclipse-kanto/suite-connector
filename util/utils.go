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

package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// NormalizeTopic convert command topics to short form
func NormalizeTopic(topic string) string {
	if strings.HasPrefix(topic, "command/") {
		topic = strings.Replace(topic, "command", "c", 1)
	}

	if strings.HasSuffix(topic, "/req/#") {
		topic = strings.Replace(topic, "/req/#", "/q/#", 1)
	}
	return topic
}

// ParseCmdTopic parses a MQTT message topic.
func ParseCmdTopic(topic string) (string, string, string) {
	if strings.HasPrefix(topic, "c//") {
		end := 3 + strings.IndexRune(topic[3:], '/')

		cmdType := topic[end+1 : end+2]

		prefix := topic[3:end]
		suffix := topic[end+3:]

		return cmdType, prefix, suffix
	}

	if strings.HasPrefix(topic, "command//") {
		end := 9 + strings.IndexRune(topic[9:], '/')

		cmdType := topic[end+1 : end+4]

		prefix := topic[9:end]
		suffix := topic[end+5:]

		return cmdType, prefix, suffix
	}

	return "", "", ""
}

// ResponseStatusTopic builds the response status topic, e.g. for a received inbox topic
// "command///req/${req-id}/response" the reply should be "command///res/${req-id}/${status}".
func ResponseStatusTopic(inboxTopic string, status int) string {
	_, prefix, suffix := ParseCmdTopic(inboxTopic)

	end := strings.IndexRune(suffix, '/')
	return fmt.Sprintf("command//%s/res/%s/%v", prefix, suffix[:end], status)
}

// NewHonoUserName returns the Hono connection user name.
func NewHonoUserName(deviceAuthID, hubTenantID string) string {
	return deviceAuthID + "@" + hubTenantID
}

// ValidPath returns error if the provided filename is not a valid one.
func ValidPath(filename string) error {
	if len(filename) == 0 {
		return errors.New("empty path")
	}

	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}

// FileExists returns if the file with provided name exists.
func FileExists(filename string) bool {
	if len(filename) == 0 {
		return false
	}

	info, err := os.Stat(filename)
	if info == nil || os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// ParseTimeout converts timeout from string to time.Duration.
func ParseTimeout(timeout string) time.Duration {
	l := len(timeout)

	if l > 0 {
		t := time.Duration(-1)
		switch timeout[l-1] {
		case 'm':
			if i, err := strconv.Atoi(timeout[:l-1]); err == nil {
				t = time.Duration(i) * time.Minute
			}
		case 's':
			if timeout[l-2] == 'm' {
				if i, err := strconv.Atoi(timeout[:l-2]); err == nil {
					t = time.Duration(i) * time.Millisecond
				}
			} else {
				if i, err := strconv.Atoi(timeout[:l-1]); err == nil {
					t = time.Duration(i) * time.Second
				}
			}
		default:
			if i, err := strconv.Atoi(timeout); err == nil {
				t = time.Duration(i) * time.Second
			}
		}

		if inTimeoutRange(t) {
			return t
		}
	}

	return 60 * time.Second
}

func inTimeoutRange(timeout time.Duration) bool {
	return timeout >= 0 && timeout < time.Hour
}
