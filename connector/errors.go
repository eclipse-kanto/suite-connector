// Copyright (c) 2022 Contributors to the Eclipse Foundation
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
	"github.com/pkg/errors"
)

var (
	// ErrClosed defines a closed publisher or subscriber channel error.
	ErrClosed = errors.New("publisher/subscriber closed")
	// ErrTimeout defines a publish timeout error.
	ErrTimeout = errors.New("publish timeout")
	// ErrNotConnected defines error on not connected state.
	ErrNotConnected = errors.New("not connected")
)
