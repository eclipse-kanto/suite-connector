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

package flags_test

import (
	"flag"
	"os"
	"testing"

	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/imdario/mergo"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/flags"
	"github.com/eclipse-kanto/suite-connector/testutil"

	"github.com/stretchr/testify/require"
)

func TestFlagsMappings(t *testing.T) {
	l := testutil.NewLogger("flags", logger.INFO)

	f := flag.NewFlagSet("testing", flag.ContinueOnError)

	cmd := new(config.Settings)
	cmd.ProvisioningFile = "provisioning.json"
	cmd.AutoProvisioningEnabled = true
	flags.Add(f, cmd)
	configFile := flags.AddGlobal(f)

	args := []string{
		"-configFile=cfg.json",
		"-provisioningFile=",
		"-deviceId=A",
		"-deviceIdPattern=B",
		"-tenantId=C",
		"-policyId=D",
		"-address=mqtts://mqtt.bosch-iot-hub.com:8883",
		"-password=E",
		"-clientId=F",
		"-localAddress=tcp://localhost:1883",
		"-localUsername=G",
		"-localPassword=H",
		"-authId=I",
		"-cacert=J",
		"-cert=K",
		"-key=L",
		"-tpmKey=M",
		"-tpmKeyPub=N",
		"-tpmHandle=0x1234",
		"-tpmDevice=O",
		"-logFile=P",
		"-logLevel=TRACE",
		"-logFileSize=10",
		"-logFileCount=100",
		"-logFileMaxAge=1000",
	}

	require.NoError(t, flags.Parse(f, args, "0.0.0", os.Exit))
	assert.Equal(t, "cfg.json", *configFile)
	assert.Equal(t, "", cmd.ProvisioningFile)
	assert.Equal(t, "A", cmd.DeviceID)
	assert.Equal(t, "B", cmd.DeviceIDPattern)
	assert.Equal(t, "C", cmd.TenantID)
	assert.Equal(t, "D", cmd.PolicyID)
	assert.Equal(t, "mqtts://mqtt.bosch-iot-hub.com:8883", cmd.Address)
	assert.Equal(t, "E", cmd.Password)
	assert.Equal(t, "F", cmd.ClientID)
	assert.Equal(t, "tcp://localhost:1883", cmd.LocalAddress)
	assert.Equal(t, "G", cmd.LocalUsername)
	assert.Equal(t, "H", cmd.LocalPassword)
	assert.Equal(t, "I", cmd.AuthID)
	assert.Equal(t, "J", cmd.CACert)
	assert.Equal(t, "K", cmd.Cert)
	assert.Equal(t, "L", cmd.Key)
	assert.Equal(t, "M", cmd.TPMKey)
	assert.Equal(t, "N", cmd.TPMKeyPub)
	assert.EqualValues(t, 0x1234, cmd.TPMHandle)
	assert.Equal(t, "O", cmd.TPMDevice)
	assert.Equal(t, "P", cmd.LogFile)
	assert.Equal(t, logger.TRACE, cmd.LogLevel)
	assert.EqualValues(t, 10, cmd.LogFileSize)
	assert.EqualValues(t, 100, cmd.LogFileCount)
	assert.EqualValues(t, 1000, cmd.LogFileMaxAge)

	flags.ConfigCheck(l, *configFile)

	m := flags.Copy(f)

	cp := config.DefaultSettings()
	if err := mergo.Map(cp, m, mergo.WithOverwriteWithEmptyValue); err != nil {
		require.NoError(t, err)
	}

	assert.Equal(t, cmd, cp)
}

func TestVersionParse(t *testing.T) {
	exitCall := false
	exit := func(_ int) {
		exitCall = true
	}

	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	cmd := new(config.Settings)
	flags.Add(f, cmd)

	args := []string{
		"-version",
	}

	require.NoError(t, flags.Parse(f, args, "0.0.0", exit))
	require.True(t, exitCall)
}

func TestInvalidFlag(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	cmd := new(config.Settings)
	flags.Add(f, cmd)

	args := []string{
		"-invalid",
	}

	require.Error(t, flags.Parse(f, args, "0.0.0", os.Exit))
}
