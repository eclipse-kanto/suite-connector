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

package cache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.uber.org/goleak"

	"github.com/eclipse-kanto/suite-connector/cache"
)

func TestCachingSimple(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	assert.Equal(t, 0, c.Size())

	v, ok := c.First()
	assert.False(t, ok)
	assert.Nil(t, v)

	v, ok = c.Get("key")
	assert.False(t, ok)
	assert.Nil(t, v)

	c.Put("key", "value", 1*time.Second)
	assert.Equal(t, 1, c.Size())

	v, ok = c.First()
	assert.True(t, ok)
	assert.Equal(t, "value", v)

	v, ok = c.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", v)

	time.Sleep(3 * time.Second)

	v, ok = c.First()
	assert.Equal(t, 0, c.Size())
	assert.False(t, ok)
	assert.Nil(t, v)

	v, ok = c.Get("key")
	assert.False(t, ok)
	assert.Nil(t, v)
	assert.Equal(t, 0, c.Size())
}

func TestMultipleExpirations(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	c.Put("key1", "value1", 500*time.Millisecond)
	c.Put("key2", "value2", 600*time.Millisecond)
	c.Put("key3", "value3", 700*time.Millisecond)
	assert.Equal(t, 3, c.Size())

	time.Sleep(2 * time.Second)
	assert.Equal(t, 0, c.Size())
}

func TestPriorityQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	c.Put("key3", "value3", 30*time.Second)
	c.Put("key2", "value2", 20*time.Second)
	c.Put("key1", "value1", 10*time.Second)

	assert.Equal(t, 3, c.Size())

	v, ok := c.First()
	assert.True(t, ok)
	assert.Equal(t, "value1", v)
	c.Remove("key1")
	assert.Equal(t, 2, c.Size())

	v, ok = c.First()
	assert.True(t, ok)
	assert.Equal(t, "value2", v)
	c.Remove("key2")
	assert.Equal(t, 1, c.Size())

	v, ok = c.First()
	assert.True(t, ok)
	assert.Equal(t, "value3", v)
	c.Remove("key3")
	assert.Equal(t, 0, c.Size())
}

func TestItemUpdate(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	c.Put("key", "value_old", 10*time.Second)
	c.Put("key", "value_new", 20*time.Second)

	v, ok := c.First()
	assert.True(t, ok)
	assert.Equal(t, "value_new", v)
	c.Remove("key")
	assert.Equal(t, 0, c.Size())
}

func TestCacheClosed(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	c.Put("key", "value", 10*time.Second)

	c.Close()

	assert.Equal(t, -1, c.Size())

	_, ok := c.First()
	assert.False(t, ok)

	_, ok = c.Get("key")
	assert.False(t, ok)

	assert.False(t, c.Put("key", "value", 10*time.Second))

	assert.False(t, c.Remove("key"))
}

func TestDoubleClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	c.Close()

	assert.NotPanics(t, func() { c.Close() }, "Close panics")
}

func TestZeroTtl(t *testing.T) {
	const items = 1700

	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	for i := 0; i < items; i++ {
		c.Put("key", "value", 0)
	}

	assert.LessOrEqual(t, c.Size(), items)
}

func TestNegativeTtl(t *testing.T) {
	defer goleak.VerifyNone(t)

	c := cache.NewTTLCache()
	defer c.Close()

	assert.Panics(t, func() { c.Put("key", "value", -time.Second) }, "No negative ttl panic")
}
