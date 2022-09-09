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

//go:build (integration && ignore) || !unit
// +build integration,ignore !unit

package config_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"

	conn "github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/stretchr/testify/require"
)

func TestLocalConnect(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	cfg.ConnectTimeout = 5 * time.Second

	logger := testutil.NewLogger("connections", logger.DEBUG, t)

	client, err := conn.NewMQTTConnection(cfg, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, config.LocalConnect(ctx, client, logger))
	defer client.Disconnect()
}

func TestDummyHonoConnect(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg, err := testutil.NewLocalConfig()
	require.NoError(t, err)
	cfg.ConnectTimeout = 5 * time.Second

	logger := testutil.NewLogger("connections", logger.DEBUG, t)

	client, err := conn.NewMQTTConnection(cfg, watermill.NewShortUUID(), logger)
	require.NoError(t, err)

	statusPub := gochannel.NewGoChannel(
		gochannel.Config{
			Persistent:          true,
			OutputChannelBuffer: int64(4),
		},
		logger,
	)
	defer statusPub.Close()

	sigs := make(chan os.Signal, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		<-ctx.Done()

		sigs <- syscall.SIGTERM
	}()

	require.NoError(t, config.HonoConnect(sigs, statusPub, client, logger))
	client.Disconnect()

	require.NoError(t, config.HonoConnect(nil, statusPub, client, logger))
	client.Disconnect()
}
