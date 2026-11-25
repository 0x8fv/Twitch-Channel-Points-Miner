package miner

import (
	"testing"
	"time"
)

func TestFormatHelpers(t *testing.T) {
	if got := formatChannelPoints(999); got != "999" {
		t.Fatalf("channel points base got %s", got)
	}
	if got := formatChannelPoints(1500); got != "1.5k" {
		t.Fatalf("channel points thousands got %s", got)
	}
	if got := formatChannelPoints(1500000); got != "1.5M" {
		t.Fatalf("channel points millions got %s", got)
	}

	if got := formatPointsWithSuffix(1250000, 1_000_000, "M"); got != "1.25M" {
		t.Fatalf("suffix format got %s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	dur := 24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second
	if got := formatDuration(dur); got != "1d 02h 03m 04s" {
		t.Fatalf("duration format got %q", got)
	}
	if got := formatDuration(45 * time.Second); got != "45s" {
		t.Fatalf("short duration format got %q", got)
	}
}
