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

package main

import (
	"os"
	"syscall"

	"github.com/pkg/errors"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/cmd/connector/app"
	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

// Launcher contains the launch system data.
type launcher struct {
	statusPub message.Publisher

	localClient *conn.MQTTConnection
	manager     conn.SubscriptionManager

	done    chan bool
	signals chan os.Signal
}

func newLauncher(client *conn.MQTTConnection, pub message.Publisher, manager conn.SubscriptionManager) app.Launcher {
	return &launcher{
		statusPub:   pub,
		manager:     manager,
		localClient: client,
		done:        make(chan bool, 2),
		signals:     make(chan os.Signal, 2),
	}
}

func (l *launcher) Run(
	cleanSession bool,
	global config.SettingsAccessor,
	args map[string]interface{},
	logger logger.Logger,
) error {
	settings := global.DeepCopy().(*config.Settings)

	if err := config.ApplySettings(cleanSession, settings, args, l.statusPub, logger); err != nil {
		return err
	}

	router := app.NewRouter(logger)

	generic := settings.HubConnectionSettings.Generic()

	cloudClient, err := config.CreateCloudConnection(settings.LocalConnection(), false, logger)
	if err != nil {
		return errors.Wrap(err, "cannot create mosquitto connection")
	}

	honoClient, cleanup, err := config.CreateHubConnection(settings.HubConnection(), false, logger)
	if err != nil {
		routing.SendStatus(routing.StatusConnectionError, l.statusPub, logger)
		return errors.Wrap(err, "cannot create Hub connection")
	}

	if !generic {
		l.manager.ForwardTo(honoClient)
	}

	paramsPub := conn.NewPublisher(l.localClient, conn.QosAtMostOnce, logger, nil)
	paramsSub := conn.NewSubscriber(cloudClient, conn.QosAtMostOnce, false, logger, nil)

	params := routing.NewGwParams(settings.DeviceID, settings.TenantID, settings.PolicyID)
	routing.ParamsBus(router, params, paramsPub, paramsSub, logger)

	reqCache := cache.NewTTLCache()

	honoPub := config.NewHonoPub(logger, honoClient)
	honoSub := config.NewHonoSub(logger, honoClient)

	mosquittoSub := conn.NewSubscriber(cloudClient, conn.QosAtLeastOnce, false, logger, nil)

	routing.EventsBus(router,
		honoPub, mosquittoSub,
		settings.TenantID, settings.DeviceID, generic,
	)
	routing.TelemetryBus(router,
		honoPub, mosquittoSub,
		settings.TenantID, settings.DeviceID, generic,
	)

	routing.CommandsResBus(router,
		honoPub, mosquittoSub, reqCache,
		settings.TenantID, settings.DeviceID, generic,
	)
	routing.CommandsReqBus(router,
		conn.NewPublisher(cloudClient, conn.QosAtLeastOnce, logger, nil),
		honoSub, reqCache, settings.TenantID, settings.DeviceID, generic,
	)

	shutdown := func(r *message.Router) error {
		go func() {
			defer func() {
				routing.SendStatus(routing.StatusConnectionClosed, l.statusPub, logger)

				reqCache.Close()

				if cleanup != nil {
					cleanup()
				}

				logger.Info("Messages router stopped", nil)
				l.done <- true
			}()

			<-r.Running()

			statusHandler := &routing.ConnectionStatusHandler{
				Pub:    l.statusPub,
				Logger: logger,
			}
			cloudClient.AddConnectionListener(statusHandler)
			defer cloudClient.RemoveConnectionListener(statusHandler)

			reconnectHandler := &routing.ReconnectHandler{
				Pub:    paramsPub,
				Params: params,
				Logger: logger,
			}
			cloudClient.AddConnectionListener(reconnectHandler)
			defer cloudClient.RemoveConnectionListener(reconnectHandler)

			connHandler := &routing.CloudConnectionHandler{
				CloudClient: cloudClient,
				Logger:      logger,
			}
			honoClient.AddConnectionListener(connHandler)
			defer honoClient.RemoveConnectionListener(connHandler)

			errorsHandler := &routing.ErrorsHandler{
				StatusPub: l.statusPub,
				Logger:    logger,
			}
			honoClient.AddConnectionListener(errorsHandler)
			defer honoClient.RemoveConnectionListener(errorsHandler)

			if err := config.HonoConnect(l.signals, l.statusPub, honoClient, logger); err != nil {
				app.StopRouter(r)
				return
			}
			defer honoClient.Disconnect()

			<-l.signals

			//Remove all subscriptions for the gateway commands
			l.manager.RemoveAll()

			honoClient.RemoveConnectionListener(errorsHandler)
			honoClient.RemoveConnectionListener(connHandler)
			cloudClient.RemoveConnectionListener(reconnectHandler)
			cloudClient.RemoveConnectionListener(statusHandler)

			cloudClient.Disconnect()

			app.StopRouter(r)
		}()

		return nil
	}
	router.AddPlugin(shutdown)

	app.StartRouter(router)

	return nil
}

func (l *launcher) Stop() {
	if l == nil {
		return
	}

	l.signals <- syscall.SIGTERM

	<-l.done
}
