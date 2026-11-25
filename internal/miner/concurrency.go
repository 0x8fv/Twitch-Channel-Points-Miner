package miner

import (
	"sync"
)

// streamerConcurrency is how many streamers are asked about at once: enough to
// make a long list quick, few enough to look like one person's browser.
const streamerConcurrency = 4

// forEachIndex calls fn(0) … fn(n-1), at most limit at a time, and waits for all of them.
func forEachIndex(n, limit int, fn func(i int)) {
	if limit < 1 {
		limit = 1
	}
	slots := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			fn(i)
		}(i)
	}
	wg.Wait()
}
