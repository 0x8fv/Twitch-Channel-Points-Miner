package miner

import (
	"strings"
	"testing"
	"time"
)

func TestNewSessionIDShape(t *testing.T) {
	id := newSessionID()
	if id == "" {
		t.Fatalf("id should not be empty")
	}
	if strings.Count(id, "-") != 4 {
		t.Fatalf("expected 4 hyphens got %d (%s)", strings.Count(id, "-"), id)
	}
}

func TestSleepWithStop(t *testing.T) {
	m := &Miner{}
	stop := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		done <- m.sleepWithStop(150*time.Millisecond, stop)
	}()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	if !<-done {
		t.Fatalf("expected sleep to abort on stop")
	}

	start := time.Now()
	if m.sleepWithStop(50*time.Millisecond, make(chan struct{})) {
		t.Fatalf("expected no stop signal")
	}
	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("sleep returned too early: %s", elapsed)
	}
}
