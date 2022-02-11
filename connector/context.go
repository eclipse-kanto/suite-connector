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
	"context"
)

type contextKey int

const (
	topicContextKey contextKey = 1 + iota
	qosContextKey
	retainContextKey
)

// SetTopicToCtx adds the topic to the provided context.
func SetTopicToCtx(ctx context.Context, topic string) context.Context {
	return context.WithValue(ctx, topicContextKey, topic)
}

// TopicFromCtx returns the topic.
func TopicFromCtx(ctx context.Context) (string, bool) {
	topic, ok := ctx.Value(topicContextKey).(string)
	return topic, ok
}

// SetQosToCtx adds the QoS to the context.
func SetQosToCtx(ctx context.Context, qos Qos) context.Context {
	if q, ok := ctx.Value(qosContextKey).(Qos); ok {
		if q != qos {
			return context.WithValue(ctx, qosContextKey, qos)
		}
	}
	return context.WithValue(ctx, qosContextKey, qos)
}

// QosFromCtx returns the QoS.
func QosFromCtx(ctx context.Context) (Qos, bool) {
	qos, ok := ctx.Value(qosContextKey).(Qos)
	return qos, ok
}

// SetRetainToCtx adds the retain value to the context.
func SetRetainToCtx(ctx context.Context, retain bool) context.Context {
	if r, ok := ctx.Value(retainContextKey).(bool); ok {
		if r != retain {
			return context.WithValue(ctx, retainContextKey, retain)
		}
	} else {
		if retain {
			return context.WithValue(ctx, retainContextKey, retain)
		}
	}
	return ctx
}

// RetainFromCtx returns the retain value.
func RetainFromCtx(ctx context.Context) bool {
	if retain, ok := ctx.Value(retainContextKey).(bool); ok {
		return retain
	}
	return false
}
