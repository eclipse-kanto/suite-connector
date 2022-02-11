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
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/fsnotify/fsnotify"
)

// SetupProvisioningWatcher creates a file watcher for the provided path.
func SetupProvisioningWatcher(path string) (*fsnotify.Watcher, error) {
	if err := ValidPath(path); err != nil {
		return nil, errors.Wrapf(err, "invalid path '%s'", path)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create watcher")
	}

	configDir := filepath.Dir(path)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, errors.Wrapf(err, "path '%s' watch error", path)
	}

	return watcher, nil
}
