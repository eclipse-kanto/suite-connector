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
	"strings"

	"github.com/pkg/errors"
)

// StringSliceV represents slice of strings flag value.
type StringSliceV struct {
	value *[]string

	provided bool
}

// NewStringSliceV creates new flag variable for slice of strings definition.
func NewStringSliceV(setter *[]string) *StringSliceV {
	return &StringSliceV{
		value: setter,
	}
}

// Provided returns true if the flag value is provided.
func (f *StringSliceV) Provided() bool {
	return f.provided
}

// String returns the flag string value.
func (f *StringSliceV) String() string {
	if f.value == nil {
		return ""
	}
	return strings.Join(*f.value, " ")
}

// Get returns the flag converted value.
func (f *StringSliceV) Get() interface{} {
	v := make([]string, 0)
	if f.value != nil {
		v = append(v, *f.value...)
	}
	return v
}

// Set validates and applies the provided value if no error.
func (f *StringSliceV) Set(value string) error {
	if len(value) == 0 {
		return errors.New("value cannot be empty")
	}
	f.provided = true
	*f.value = strings.Fields(value)
	return nil
}
