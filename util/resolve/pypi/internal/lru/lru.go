// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// package lru provides a generic least-recently-used cache.
package lru

import (
	"fmt"
)

// Cache implements an LRU cache, with a particular maximum size.
type Cache[K comparable, V any] struct {
	m       map[K]*listNode[cacheEntry[K, V]]
	l       *list[cacheEntry[K, V]]
	maxSize int
}

type cacheEntry[K, V any] struct {
	k K
	v V
}

func New[K comparable, V any](size int) *Cache[K, V] {
	return &Cache[K, V]{
		m:       make(map[K]*listNode[cacheEntry[K, V]], size+1),
		l:       new(list[cacheEntry[K, V]]),
		maxSize: size,
	}
}

// Add inserts an element into the cache, removing an element if necessary to
// keep the size fixed. If the key is already present its value is updated.
func (c *Cache[K, V]) Add(k K, v V) {
	if ln, ok := c.m[k]; ok {
		ln.value.v = v
		c.l.MoveToFront(ln)
		// No change in size.
		return
	}

	if len(c.m) < c.maxSize {
		// The key is new, and there is space in the cache.
		c.m[k] = c.l.Push(cacheEntry[K, V]{k: k, v: v})
		return
	}
	// We have to delete something, reuse the tail node to avoid an
	// allocation.
	ln := c.l.tail
	delete(c.m, ln.value.k)
	ln.value.k = k
	ln.value.v = v
	c.m[k] = ln
	c.l.MoveToFront(ln)
}

// Get returns the value associated with the given key from the cache, as well
// as a boolean indicating whether the key was found.
// It also moves the accessed item to the front of the LRU list, indicating it
// was recently used.
func (c *Cache[K, V]) Get(k K) (v V, ok bool) {
	ln, ok := c.m[k]
	if !ok {
		return v, false
	}
	c.l.MoveToFront(ln)
	return ln.value.v, true
}

// list is a doubly-linked list.
type list[T any] struct {
	head, tail *listNode[T]
}

// listNode is a single element in a list
type listNode[T any] struct {
	value T

	prev, next *listNode[T]
}

// Push inserts a new element at the front of the list. It returns the listNode
// that was added.
func (l *list[T]) Push(v T) *listNode[T] {
	n := &listNode[T]{value: v, next: l.head}
	if l.head != nil {
		l.head.prev = n
	}
	l.head = n
	if l.tail == nil {
		l.tail = n
	}
	return l.head
}

// MoveToFront moves the provided listNode to the front of the list. n is
// assumed to already be an element of the list.
func (l *list[T]) MoveToFront(n *listNode[T]) {
	if n == l.head {
		return
	}
	if n == l.tail {
		l.tail = n.prev
	}
	n.prev.next = n.next
	if n.next != nil {
		n.next.prev = n.prev
	}
	n.prev = nil
	n.next = l.head
	l.head.prev = n
	l.head = n
}

func (l *list[T]) String() string {
	var vals []string
	for n := l.head; n != nil; n = n.next {
		vals = append(vals, fmt.Sprint(n.value))
	}
	return fmt.Sprint(vals)
}
