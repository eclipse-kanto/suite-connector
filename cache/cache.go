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

package cache

import (
	"container/heap"
	"sync"
	"time"
)

// Cache is a system used structure for the goals of caching.
type Cache struct {
	closed  bool
	control chan bool

	mutex    sync.Mutex
	queue    priorityqueue
	elements map[string]*element
}

// NewTTLCache creates cache data for the TTL usage.
func NewTTLCache() *Cache {
	c := &Cache{
		control:  make(chan bool, 8),
		elements: make(map[string]*element),
	}
	go c.startCleaner()
	return c
}

// Size returns the elements size.
func (c *Cache) Size() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return -1
	}

	return len(c.elements)
}

// First removes the first element.
func (c *Cache) First() (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil, false
	}

	return c.queue.first()
}

// Get finds and returns the element mapped with the provided key if available.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil, false
	}

	if e, ok := c.elements[key]; ok {
		return e.value, true
	}

	return nil, false
}

// Put adds element with the provided key.
func (c *Cache) Put(key string, value interface{}, ttl time.Duration) bool {
	if ttl < 0 {
		panic("ttl < 0")
	}

	notify := false
	defer c.setchanges(&notify)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return false
	}

	notify = true
	expireAt := time.Now().Add(ttl)

	if e, ok := c.elements[key]; ok {
		c.update0(e, value, ttl, expireAt)
	} else {
		e := &element{
			index:   -1,
			key:     key,
			value:   value,
			ttl:     ttl,
			expTime: expireAt,
		}

		c.add0(e)
	}

	return true
}

// Remove removes the element with the provided key and returns true is present.
func (c *Cache) Remove(key string) bool {
	notify := false
	defer c.setchanges(&notify)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return false
	}

	if e, ok := c.elements[key]; ok {
		notify = true
		c.remove0(e)
	}

	return notify
}

// Close is used to close the cache.
func (c *Cache) Close() {
	if c.doClose() {
		return
	}

	close(c.control)
}

func (c *Cache) doClose() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	result := c.closed

	c.closed = true

	return result
}

func (c *Cache) update0(e *element, value interface{}, ttl time.Duration, expTime time.Time) {
	e.ttl = ttl
	e.value = value
	e.expTime = expTime
	c.queue.heapify(e)
}

func (c *Cache) add0(e *element) {
	c.queue.heappush(e)
	c.elements[e.key] = e
}

func (c *Cache) remove0(e *element) {
	delete(c.elements, e.key)
	c.queue.heapremove(e)
}

func (c *Cache) startCleaner() {
	timer := time.NewTimer(time.Minute * 10)

loop:
	for {
		timer.Reset(c.pauseTime())

		select {
		case _, ok := <-c.control:
			if ok {
				timer.Stop()

				continue
			} else {
				break loop
			}

		case <-timer.C:
			timer.Stop()

			c.clearExpired()
		}
	}

	timer.Stop()
}

func (c *Cache) pauseTime() time.Duration {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var result time.Duration
	if c.queue.Len() > 0 {
		first := c.queue[0]
		result = time.Until(first.expTime)
		if result < 0 {
			result = time.Millisecond
		}
	} else {
		result = time.Minute * 10
	}

	return result
}

func (c *Cache) setchanges(fire *bool) {
	if *fire {
		c.control <- true
	}
}

func (c *Cache) clearExpired() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := 0; i < c.queue.Len(); i++ {
		e := c.queue[i]
		if !e.hasExpired() {
			break
		}
		c.remove0(e)
	}
}

type element struct {
	index int

	key   string
	value interface{}

	ttl     time.Duration
	expTime time.Time
}

func (e *element) hasExpired() bool {
	if e.ttl == 0 {
		return true
	}
	return e.expTime.Before(time.Now())
}

type priorityqueue []*element

func (q priorityqueue) Len() int {
	return len(q)
}

func (q priorityqueue) first() (interface{}, bool) {
	if q.Len() == 0 {
		return nil, false
	}
	return q[0].value, true
}

func (q priorityqueue) Less(i, j int) bool {
	return q[i].expTime.Before(q[j].expTime)
}

func (q priorityqueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *priorityqueue) Push(x interface{}) {
	n := len(*q)
	item := x.(*element)
	item.index = n
	*q = append(*q, item)
}

func (q *priorityqueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*q = old[0 : n-1]
	return item
}

func (q *priorityqueue) heappush(e *element) {
	heap.Push(q, e)
}

func (q *priorityqueue) heapremove(e *element) {
	heap.Remove(q, e.index)
}

func (q *priorityqueue) heapify(e *element) {
	heap.Fix(q, e.index)
}
