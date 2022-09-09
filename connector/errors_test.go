// Copyright (c) 2022 Contributors to the Eclipse Foundation
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

package connector_test

import (
	"testing"

	"github.com/pkg/errors"

	conn "github.com/eclipse-kanto/suite-connector/connector"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	type test struct {
		id  string
		err error
	}

	tests := []test{
		test{
			id:  "ErrClosed",
			err: conn.ErrClosed,
		},
		test{
			id:  "ErrTimeout",
			err: conn.ErrTimeout,
		},
		test{
			id:  "ErrNotConnected",
			err: conn.ErrNotConnected,
		},
	}

	for _, tc := range tests {
		tcId := tc.id
		err := tc.err

		t.Run(tcId, func(t *testing.T) {
			w := errors.Wrapf(err, "%s", "testing")
			assert.ErrorIs(t, w, err)
			assert.True(t, errors.Is(w, err))

			var cause error
			assert.True(t, errors.As(w, &cause))
			assert.True(t, errors.Is(cause, err))

			s := errors.WithStack(err)
			assert.ErrorIs(t, s, err)

			//Bug in multierror
			//r := multierror.Append(err, errors.New(watermill.NewShortUUID()))
			//assert.True(t, errors.Is(r, err))
		})
	}
}
