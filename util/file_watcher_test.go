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

package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eclipse-kanto/suite-connector/util"
)

func TestSetupConfigWatcherDefaultValue(t *testing.T) {
	watcher, err := util.SetupProvisioningWatcher("testing.json")
	assert.NoError(t, err)
	assert.NotNil(t, watcher)
	watcher.Close()
}

func TestSetupConfigWatcherEmptyPath(t *testing.T) {
	_, err := util.SetupProvisioningWatcher("")
	assert.Error(t, err)
}

func TestSetupConfigWatcherNonExistingPath(t *testing.T) {
	_, err := util.SetupProvisioningWatcher("non-existing/testing.json")
	assert.Error(t, err)
}
