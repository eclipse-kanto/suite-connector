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

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package connector_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/subscriber"

	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"
)

func BenchmarkSubscriber(b *testing.B) {
	logger := watermill.NopLogger{}

	config, err := testutil.NewLocalConfig()
	require.NoError(b, err)

	client, err := connector.NewMQTTConnection(config, watermill.NewShortUUID(), logger)
	require.NoError(b, err)

	future := client.Connect()
	<-future.Done()
	require.NoError(b, future.Error())
	defer client.Disconnect()

	sub := connector.NewSubscriber(client, connector.QosAtLeastOnce, true, logger, nil)
	messages, err := sub.Subscribe(context.Background(), "simple/bench/#")
	require.NoError(b, err)
	defer sub.Close()

	b.ResetTimer()

	go func() {
		pub := connector.NewPublisher(client, connector.QosAtMostOnce, logger, nil)

		for i := 0; i < b.N; i++ {
			msg := message.NewMessage("bench", []byte("bench"))
			if err := pub.Publish("simple/bench/demo", msg); err != nil {
				panic(err)
			}
		}
	}()

	if _, ok := subscriber.BulkRead(messages, b.N, time.Second*60); !ok {
		require.Fail(b, "Messages not delivered")
	}
}
