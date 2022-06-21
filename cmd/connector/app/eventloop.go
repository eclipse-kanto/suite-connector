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

package app

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse-kanto/suite-connector/util"

	conn "github.com/eclipse-kanto/suite-connector/connector"
)

const (
	fsOp = fsnotify.Create | fsnotify.Write | fsnotify.Chmod | fsnotify.Remove | fsnotify.Rename
)

// LaunchFactory returns new launcher instance
type LaunchFactory func(client *conn.MQTTConnection, pub message.Publisher, manager conn.SubscriptionManager) Launcher

// Launcher interface enables app launching.
type Launcher interface {
	// Run triggers the launch process.
	Run(cleanSession bool, settings config.SettingsAccessor, cli map[string]interface{}, logger logger.Logger) error

	// Stop invokes launcher exit.
	Stop()
}

type mockLauncher struct{}

func (l *mockLauncher) Run(
	cleanSession bool,
	global config.SettingsAccessor,
	cli map[string]interface{},
	logger logger.Logger,
) error {
	//Nothing to do
	return nil
}

func (l *mockLauncher) Stop() {
	//Nothing to do
}

//NewMockLauncher creates mock launcher for testing purposes
func NewMockLauncher(client *conn.MQTTConnection, pub message.Publisher, manager conn.SubscriptionManager) Launcher {
	return new(mockLauncher)
}

// NewRouter creates new messages router
func NewRouter(logger logger.Logger) *message.Router {
	router, _ := message.NewRouter(message.RouterConfig{}, logger)
	config.SetupTracing(router, logger)

	logger.Info("Starting messages router...", nil)

	return router
}

// StartRouter starts router instance.
func StartRouter(r *message.Router) {
	go func() {
		if err := r.Run(context.Background()); err != nil {
			r.Logger().Error("Failed to create cloud router", err, nil)
		}
	}()
}

// StopRouter stops router instance.
func StopRouter(r *message.Router) {
	_ = r.Close()
}

// Run runs the app.
func Run(
	ctx context.Context,
	factory LaunchFactory,
	settings config.SettingsAccessor,
	cli map[string]interface{},
	logger logger.Logger,
) error {
	localClient, err := config.CreateLocalConnection(settings.LocalConnection(), logger)
	if err != nil {
		return errors.Wrap(err, "cannot create mosquitto connection")
	}
	if err := config.LocalConnect(ctx, localClient, logger); err != nil {
		return errors.Wrap(err, "cannot connect to mosquitto")
	}

	statusPub := conn.NewPublisher(localClient, conn.QosAtLeastOnce, logger, nil)
	defer statusPub.Close()

	watcher, err := util.SetupProvisioningWatcher(settings.Provisioning())
	if err != nil {
		defer localClient.Disconnect()

		routing.SendStatus(routing.StatusConfigError, statusPub, logger)

		return errors.Wrap(err, "file watcher error")
	}
	defer watcher.Close()
	logger.Debugf("Provisioning file watcher created for %s", settings.Provisioning())

	manager := conn.NewSubscriptionManager()
	router := routing.CreateServiceRouter(localClient, manager, logger)
	defer router.Close()

	defer localClient.Disconnect()

	launcher := factory(localClient, statusPub, manager)
	if err := launcher.Run(false, settings, cli, logger); err != nil {
		logger.Error("Failed to create message bus", err, nil)
		launcher = nil
	}

	launch := func(e fsnotify.Event) {
		logger.Infof("[fsnotify] event %v", e)

		if e.Op&fsOp == 0 {
			return
		}

		stopLauncher(launcher)

		launcher = factory(localClient, statusPub, manager)
		if err := launcher.Run(true, settings, cli, logger); err != nil {
			logger.Errorf("Failed to create message bus: %v", err)
			launcher = nil
		}
	}
	EventLoop(ctx, watcher.Events, settings.Provisioning(), launch)

	stopLauncher(launcher)

	return nil
}

// EventLoop main event loop.
func EventLoop(ctx context.Context, watchEvents chan fsnotify.Event, fileName string, launch func(fsnotify.Event)) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	events := make([]fsnotify.Event, 0)
	provisioningName := filepath.Base(fileName)

loop:
	for {
		select {
		case <-sigs:
			break loop

		case <-ctx.Done():
			break loop

		case event := <-watchEvents:
			if filepath.Base(event.Name) == provisioningName {
				events = append([]fsnotify.Event{event}, events...)
			}

		case <-ticker.C:
			if len(events) == 0 {
				continue loop
			}

			for _, event := range events {
				launch(event)
				break
			}

			events = make([]fsnotify.Event, 0)
		}
	}
}

func stopLauncher(l Launcher) {
	if l == nil {
		return
	}

	v := reflect.ValueOf(l)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}

	l.Stop()
}
