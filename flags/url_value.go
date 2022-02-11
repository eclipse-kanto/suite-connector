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

package flags

import (
	"net/url"

	"github.com/pkg/errors"
)

// URLV represents URL flag value.
type URLV struct {
	setter *string

	provided bool

	value string
}

// NewURLV creates new flag variable for URL definition.
func NewURLV(setter *string, defaultVal string) *URLV {
	return &URLV{
		setter: setter,
		value:  defaultVal,
	}
}

// Provided returns true if the flag value is provided.
func (f *URLV) Provided() bool {
	return f.provided
}

// String returns the flag string value.
func (f *URLV) String() string {
	return f.value
}

// Get returns the flag converted value.
func (f *URLV) Get() interface{} {
	return f.value
}

// Set validates and applies the provided value if no error.
func (f *URLV) Set(value string) error {
	if _, err := url.ParseRequestURI(value); err != nil {
		return errors.New("invalid URL")
	}

	f.provided = true
	f.value = value
	*f.setter = value

	return nil
}
