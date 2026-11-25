package miner

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestForEachIndexVisitsEveryIndexWithinTheLimit(t *testing.T) {
	const n, limit = 25, 4
	var running, peak atomic.Int32
	var mu sync.Mutex
	seen := make(map[int]int)
	forEachIndex(n, limit, func(i int) {
		now := running.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		seen[i]++
		mu.Unlock()
		running.Add(-1)
	})
	if len(seen) != n {
		t.Fatalf("visited %d indexes, want %d", len(seen), n)
	}
	for i, count := range seen {
		if count != 1 {
			t.Fatalf("index %d visited %d times", i, count)
		}
	}
	if got := peak.Load(); got > limit || got < 2 {
		t.Fatalf("ran %d at once, want more than one and at most %d", got, limit)
	}
}

func TestForEachIndexHandlesEmptyAndBadLimit(t *testing.T) {
	forEachIndex(0, 4, func(int) { t.Fatalf("nothing to visit") })
	calls := 0
	forEachIndex(3, 0, func(int) { calls++ })
	if calls != 3 {
		t.Fatalf("a non-positive limit should still visit everything one at a time, got %d", calls)
	}
}
