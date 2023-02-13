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

package flags

import (
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/util"
)

const (
	flagCACert = "caCert"
)

var (
	// ErrParse is generic flags parse error
	ErrParse = errors.New("cannot parse flags")
)

// Add adds the Suite Connector flags and uses the provided settings to collect the provided values.
func Add(f *flag.FlagSet, settings *config.Settings) {
	def := config.DefaultSettings()

	AddLog(f, &settings.LogSettings, &def.LogSettings)
	AddHub(f, &settings.HubConnectionSettings, &def.HubConnectionSettings)
	AddLocalBroker(f, &settings.LocalConnectionSettings, &def.LocalConnectionSettings)

	f.StringVar(&settings.ProvisioningFile,
		"provisioningFile", def.ProvisioningFile,
		"Provisioning `file` in JSON format",
	)
}

// AddGlobal adds the Suite Connector global flags.
func AddGlobal(f *flag.FlagSet) (configFile *string) {
	configFile = f.String("configFile", "", "Configuration `file` in JSON format with flags values")
	return
}

// ConfigCheck checks for config file existence.
func ConfigCheck(logger logger.Logger, configFile string) {
	if len(configFile) > 0 && !util.FileExists(configFile) {
		logger.Warnf("Provided configuration file %q does not exist", configFile)
	}
}

// AddLog adds the Logger related flags.
func AddLog(f *flag.FlagSet, settings, def *logger.LogSettings) {
	f.Var(NewLogLevelV(&settings.LogLevel, def.LogLevel),
		"logLevel",
		"Log `level`s are ERROR, WARN, INFO, DEBUG, TRACE",
	)
	f.IntVar(&settings.LogFileSize,
		"logFileSize", def.LogFileSize,
		"Log file `size` in MB before it gets rotated",
	)
	f.IntVar(&settings.LogFileCount,
		"logFileCount", def.LogFileCount,
		"Log file max rotations `count`",
	)
	f.IntVar(&settings.LogFileMaxAge,
		"logFileMaxAge", def.LogFileMaxAge,
		"Log file rotations max age in `days`",
	)
	f.StringVar(&settings.LogFile,
		"logFile", def.LogFile,
		"Log `file` location",
	)
}

// AddHub adds the Hub connection related flags.
func AddHub(f *flag.FlagSet, settings, def *config.HubConnectionSettings) {
	f.Var(NewURLV(&settings.Address, def.Address), "address", "Hub endpoint `url`")

	f.StringVar(&settings.DeviceIDPattern,
		"deviceIdPattern", def.DeviceIDPattern,
		"Device ID `Pattern`",
	)

	f.StringVar(&settings.DeviceID, "deviceId", def.DeviceID, "Device `ID`")
	f.StringVar(&settings.TenantID, "tenantId", def.TenantID, "Tenant `ID`")
	f.StringVar(&settings.PolicyID, "policyId", def.PolicyID, "Policy `ID`")
	f.StringVar(&settings.AuthID, "authId", def.AuthID, "Authorization `ID`")
	f.StringVar(&settings.Password, "password", def.Password, "Hub endpoint password")
	f.StringVar(&settings.Username, "username", def.Username, "Hub endpoint username")
	f.StringVar(&settings.ClientID, "clientId", def.ClientID, "Hub client `ID`")

	AddTLS(f, &settings.TLSSettings, &def.TLSSettings)
}

// AddTLS add the TLS connection related flags.
func AddTLS(f *flag.FlagSet, settings, def *config.TLSSettings) {
	f.StringVar(&settings.CACert,
		flagCACert, def.CACert,
		"A PEM encoded CA certificates `file`",
	)
	f.StringVar(&settings.Cert,
		"cert", def.Cert,
		"A PEM encoded certificate `file` for cloud access",
	)
	f.StringVar(&settings.Key,
		"key", def.Key,
		"A PEM encoded unencrypted private key `file` for cloud access",
	)
	f.StringVar(&settings.TPMKey,
		"tpmKey", def.TPMKey,
		"Private part of TPM2 key `file`",
	)
	f.StringVar(&settings.TPMKeyPub,
		"tpmKeyPub", def.TPMKeyPub,
		"Public part of TPM2 key `file`",
	)
	f.StringVar(&settings.TPMDevice,
		"tpmDevice", def.TPMDevice,
		"Path to the device `file` or the unix socket to access the TPM2",
	)
	f.Uint64Var(&settings.TPMHandle,
		"tpmHandle", def.TPMHandle,
		"TPM2 storage root key handle",
	)
}

// AddLocalBroker add the local connection related flags.
func AddLocalBroker(f *flag.FlagSet, settings, def *config.LocalConnectionSettings) {
	f.Var(NewURLV(&settings.LocalAddress, def.LocalAddress),
		"localAddress", "Local broker `url`",
	)
	f.StringVar(&settings.LocalUsername,
		"localUsername", def.LocalUsername,
		"Username for authorized local client",
	)
	f.StringVar(&settings.LocalPassword,
		"localPassword", def.LocalPassword,
		"Password for authorized local client",
	)

	f.StringVar(&settings.LocalCACert,
		"localCACert", def.LocalCACert,
		"A PEM encoded local broker CA certificates `file`",
	)
	f.StringVar(&settings.LocalCert,
		"localCert", def.LocalCert,
		"A PEM encoded certificate `file` for local broker",
	)
	f.StringVar(&settings.LocalKey,
		"localKey", def.LocalKey,
		"A PEM encoded unencrypted private key `file` for local broker",
	)
}

// Parse invokes flagset parse and processes the version
func Parse(f *flag.FlagSet, args []string, version string, exit func(code int)) error {
	rand.Seed(time.Now().UnixNano())

	fVersion := f.Bool("version", false, "Prints current version and exits")

	if err := f.Parse(args); err != nil {
		return err
	}

	if *fVersion {
		fmt.Println(version)
		exit(0)
	}

	return nil
}

// Copy configured all set flag values to map
func Copy(f *flag.FlagSet) map[string]interface{} {
	m := make(map[string]interface{}, f.NFlag())

	f.Visit(func(f *flag.Flag) {
		name := f.Name
		getter := f.Value.(flag.Getter)

		if name == flagCACert {
			name = "CACert"
		} else {
			name = strings.ReplaceAll(name, "Id", "ID")
			name = strings.ReplaceAll(name, "tpm", "TPM")
		}

		m[name] = getter.Get()
	})

	return m
}
