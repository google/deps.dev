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

package lru

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/golang/groupcache/lru"
)

func BenchmarkCacheGet(b *testing.B) {
	const size = 1000
	c := New[int, string](size)
	gc := lru.New(size)
	for i := 0; i < size; i++ {
		val := make([]byte, 20)
		rand.Read(val)
		c.Add(i, string(val))
		gc.Add(i, string(val))
	}
	b.Run("____Cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Around half and half hits and misses.
			v, ok := c.Get(i % (size * 2))
			_, _ = v, ok
		}
	})
	b.Run("lru.Cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Around half and half hits and misses.
			v, ok := gc.Get(i % (size * 2))
			var val string
			if ok {
				val = v.(string)
			}
			_ = val
		}
	})
}

func BenchmarkCacheAddFull(b *testing.B) {
	const size = 1000
	c := New[int, string](size)
	gc := lru.New(size)
	for i := 0; i < size; i++ {
		val := make([]byte, 20)
		rand.Read(val)
		c.Add(i, string(val))
		gc.Add(i, string(val))
	}
	value := "a value"
	b.Run("____Cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c.Add(size+i, value)
		}
	})
	b.Run("lru.Cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			gc.Add(size+i, value)
		}
	})
}

func TestCache(t *testing.T) {
	const size = 100
	c := New[int, int](size)
	// First add exactly size elements.
	for i := 0; i < size; i++ {
		c.Add(i, ^i)
	}
	for i := 0; i < size; i++ {
		j, ok := c.Get(i)
		if !ok {
			t.Fatalf("Get after %d Adds: %d not present", size, i)
		}
		if j != ^i {
			t.Fatalf("Get(%d): want %d, got: %d", i, ^i, j)
		}
	}
	// Add another 10. We've just asked for 0-size-1 in order, so 0-9 should
	// be evicted.
	for i := size; i < size+10; i++ {
		c.Add(i, ^i)
	}
	for i := 0; i < 10; i++ {
		if j, ok := c.Get(i); ok {
			t.Fatalf("Get(%d) after %d Adds: should not be present, got: %d", i, size+10, j)
		}
	}
	// Make sure Add marks things as recently used even if they already
	// exist, and updates the value.
	c.Add(10, ^0) // should be next in line for eviction.
	c.Add(0, ^0)
	if got, ok := c.Get(10); !ok {
		t.Fatal("Expect 10 to not be evicted, but it was")
	} else if got != ^0 {
		t.Fatal("Wrong value after update")
	}
}

func TestListPush(t *testing.T) {
	var (
		l    list[int]
		want []int
	)
	for i := 0; i < 10; i++ {
		ln := l.Push(i)
		if ln.value != i {
			t.Fatalf("value mismatch: want: %d, got: %d", i, ln.value)
		}
		want = append([]int{i}, want...)
	}
	var got []int
	for n := l.head; n != nil; n = n.next {
		got = append(got, n.value)
	}
	if !slices.Equal(want, got) {
		t.Fatalf("Mismatch after 10 Pushes:\nwant: %v\n got: %v", want, got)
	}
}

func TestListMoveToFront(t *testing.T) {
	var (
		l    list[int]
		want []int
	)
	for i := 0; i < 100; i++ {
		l.Push(i)
		want = append([]int{i}, want...)
	}

	pick := func() (int, *listNode[int]) {
		n := rand.Intn(len(want))
		ln := l.head
		for i := 0; i < n && ln != nil; i++ {
			ln = ln.next
		}
		if ln == nil {
			t.Fatal("not enough elements in list?")
		}
		return n, ln
	}

	for i := 0; i < 1000; i++ {
		j, ln := pick()
		if ln.value != want[j] {
			t.Fatalf("mismatch at position %d: want: %d, got %d\nslice: %v\n list: %v", j, want[j], ln.value, want, l)
		}
		// shuffle everything up to cover position j
		copy(want[1:j+1], want[:j])
		want[0] = ln.value
		l.MoveToFront(ln)
		var got []int
		for ln := l.head; ln != nil; ln = ln.next {
			got = append(got, ln.value)
		}
		if !slices.Equal(want, got) {
			t.Fatalf("mismatch after %d MoveToFront:\nwant: %v\n got: %v", i+1, want, got)
		}
	}
}
