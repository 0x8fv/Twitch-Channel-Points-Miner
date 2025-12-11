package twitchchannelpointsminer

import (
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestParseWatchPriorities(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []watchPriority
	}{
		{
			name: "defaults when empty",
			in:   nil,
			want: defaultWatchPriorities(),
		},
		{
			name: "ignores unknowns and deduplicates",
			in:   []string{"drops", "ORDER", "drops", "points_desc", "ignored"},
			want: []watchPriority{
				watchPriorityDrops,
				watchPriorityOrder,
				watchPriorityPointsDescending,
			},
		},
		{
			name: "falls back to defaults when nothing recognized",
			in:   []string{"foo", "bar"},
			want: defaultWatchPriorities(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWatchPriorities(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d got %v want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeGameList(t *testing.T) {
	got := normalizeGameList([]string{" Valorant ", "Tom Clancy's Rainbow Six Siege X", "five night's at freddys", "valorant", "   "})
	want := []string{"valorant", "tom clancy's rainbow six siege x", "five night's at freddys"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestWatchInterval(t *testing.T) {
	m := &Miner{}
	if got := m.watchInterval(0); got != 20*time.Second {
		t.Fatalf("zero count got %s", got)
	}
	if got := m.watchInterval(2); got != 10*time.Second {
		t.Fatalf("two watchers got %s", got)
	}
	if got := m.watchInterval(10); got != 5*time.Second {
		t.Fatalf("min interval cap expected 5s got %s", got)
	}
}

func TestStreakPriorityLimit(t *testing.T) {
	m := &Miner{}
	now := time.Now()
	if got := m.streakPriorityLimit(now); got != streakPriorityMinutesBase {
		t.Fatalf("zero start uses base got %f", got)
	}
	m.startedAt = now.Add(-11 * time.Hour)
	if got := m.streakPriorityLimit(now); got != streakPriorityMinutesExtended {
		t.Fatalf("long session uses extended got %f", got)
	}
}

func TestShouldPrioritizeStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
		},
	}
	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("expected streak priority when conditions met")
	}

	streamer.OfflineAt = now.Add(-10 * time.Minute)
	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("recent offline should skip streak priority")
	}
}

func TestGamePreference(t *testing.T) {
	m := &Miner{
		gamePriority:      []string{"valorant", "wow"},
		gamePriorityIndex: map[string]int{"valorant": 0, "wow": 1},
		gameExclusions:    map[string]struct{}{"banned": {}},
	}
	streamer := &entities.Streamer{
		Stream: &entities.Stream{
			Game: map[string]interface{}{"displayName": "Valorant"},
		},
	}
	rank, excluded := m.gamePreference(streamer)
	if excluded {
		t.Fatalf("expected not excluded")
	}
	if rank != 0 {
		t.Fatalf("expected rank 0 got %d", rank)
	}

	streamer.Stream.Game["displayName"] = "Banned"
	rank, excluded = m.gamePreference(streamer)
	if !excluded || rank != 0 {
		t.Fatalf("excluded game should return excluded=true rank=0 got rank=%d excluded=%t", rank, excluded)
	}
}

func TestGameInfoRespectsShowFlag(t *testing.T) {
	streamer := &entities.Streamer{
		Stream: &entities.Stream{
			Game: map[string]interface{}{"displayName": "Tom Clancy's Rainbow Six Siege X"},
		},
	}
	m := &Miner{showGameInfo: false}
	if got := m.gameInfo(streamer); got != "" {
		t.Fatalf("expected no game info when disabled got %q", got)
	}

	m.showGameInfo = true
	if got := m.gameInfo(streamer); got != "Tom Clancy's Rainbow Six Siege X" {
		t.Fatalf("expected game name when enabled got %q", got)
	}
}

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

	if got := formatDropProgress(3, 10); got != "3/10" {
		t.Fatalf("drop progress got %s", got)
	}
	if got := formatDropProgress(5, 0); got != "5" {
		t.Fatalf("drop progress with zero required got %s", got)
	}

	if got := progressPercent(5, 10); got != 50 {
		t.Fatalf("percent got %d", got)
	}
	if got := progressPercent(1, 0); got != 100 {
		t.Fatalf("zero required but progress present got %d", got)
	}
	if got := progressPercent(0, 0); got != 0 {
		t.Fatalf("zero current and required got %d", got)
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
