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

package connector

import (
	"github.com/eclipse/paho.mqtt.golang/packets"
)

//Disable paho offline buffer
type nostore struct {
	// Nothing to do
}

func (store *nostore) Open() {
	// Nothing to do
}

func (store *nostore) Put(string, packets.ControlPacket) {
	// Nothing to do
}

func (store *nostore) Get(string) packets.ControlPacket {
	// Nothing to do
	return nil
}

func (store *nostore) Del(string) {
	// Nothing to do
}

func (store *nostore) All() []string {
	return nil
}

func (store *nostore) Close() {
	// Nothing to do
}

func (store *nostore) Reset() {
	// Nothing to do
}
