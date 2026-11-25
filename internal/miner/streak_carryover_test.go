package miner

import (
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/streak"
)

func TestResolvedStreakShortRestartDoesNotPrioritizeStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}
	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved short restart should not keep streak priority")
	}
}

func TestResolvedStreakShortRestartCarryoverSurvivesStreamRefreshReset(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved short restart should stay suppressed after stream refresh reset")
	}
}

func TestResolvedStreakShortRestartCarryoverLearnsCompletionFromUpdateSync(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	streamer.Stream.WatchStreakMissing = false
	m.syncResolvedStreakState(streamer, streamer.Stream.BroadcastID, streamer.Stream.CreatedAt, true)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("completion learned from update sync should suppress short restart streak priority")
	}
}

func TestResolvedStreakShortRestartCarryoverSurvivesMultipleRestartCycles(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}
	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	if firstCarryoverUntil.IsZero() {
		t.Fatalf("first restart should arm a carryover window")
	}
	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	m.setPresence(streamer, false, "poll")
	secondOfflineAt := streamer.OfflineAt
	secondCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	wantSecondCarryoverUntil := secondOfflineAt.Add(streak.CarryoverWindow)
	if !secondCarryoverUntil.Equal(wantSecondCarryoverUntil) {
		t.Fatalf("repeated short restart should extend carryover to latest offline time, got %s want %s", secondCarryoverUntil, wantSecondCarryoverUntil)
	}

	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("carryover should continue suppressing streak priority across repeated short restarts")
	}
}

func TestResolvedStreakCarryoverPropagatesAfterOriginalExpiryAcrossShortRestarts(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &domain.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	streamer.ResolvedStreakCarryoverUntil = time.Now().Add(-time.Second)

	m.setPresence(streamer, false, "poll")
	if streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("resolved carryover should re-arm from the later offline segment")
	}

	streamer.Stream = &domain.Stream{
		BroadcastID:        "broadcast-c",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved carryover should suppress streak priority after repeated short restarts")
	}
}

func TestResolvedStreakCarryoverExpiresAfterWindow(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		ResolvedStreakCarryover:      true,
		ResolvedStreakCarryoverUntil: now.Add(-time.Second),
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-10 * time.Minute),
			CreatedAt:          now.Add(-10 * time.Minute),
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("expired carryover should no longer suppress streak priority")
	}
	if streamer.ResolvedStreakCarryover {
		t.Fatalf("expired carryover should be cleared")
	}
	if !streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("expired carryover timestamp should be cleared")
	}
}

func TestResolvedStreakCarryoverArmsAcrossOnlineBroadcastReset(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			BroadcastID:        "broadcast-2",
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-time.Minute),
			CreatedAt:          now.Add(-time.Minute),
		},
	}
	streamer.CompletedWatchStreak = true

	m.syncResolvedStreakState(streamer, "broadcast-1", now.Add(-15*time.Minute), true)

	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("online broadcast reset after a completed segment should suppress streak priority")
	}
	if !streamer.ResolvedStreakCarryover {
		t.Fatalf("online broadcast reset should arm carryover")
	}
	if streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("online broadcast reset should set carryover expiry")
	}
}

func TestResolvedStreakCarryoverLongOfflineClearsPropagation(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)
	m.setPresence(streamer, false, "poll")
	streamer.ResolvedStreakCarryoverUntil = time.Now().Add(-time.Second)

	streamer.Stream = &domain.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if streamer.CompletedWatchStreak {
		t.Fatalf("long offline gap should clear propagation signal")
	}
	if streamer.ResolvedStreakCarryover {
		t.Fatalf("expired carryover should stay cleared after long offline gap")
	}
	if !streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("expired carryover timestamp should stay cleared after long offline gap")
	}
	if !m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("new segment after long offline gap should be eligible for streak priority")
	}
}

func TestResolvedStreakCarryoverRefreshesFromFreshCompletionDuringActiveWindow(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)
	m.setPresence(streamer, false, "poll")
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	if firstCarryoverUntil.IsZero() {
		t.Fatalf("segment A should arm carryover")
	}

	streamer.Stream = &domain.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(-time.Minute),
		CreatedAt:          now.Add(-time.Minute),
	}
	m.setPresence(streamer, true, "poll")

	m.updateHistory(streamer, "WATCH_STREAK", 75)
	if !streamer.CompletedWatchStreak {
		t.Fatalf("segment B should record a fresh completion")
	}

	time.Sleep(20 * time.Millisecond)
	m.setPresence(streamer, false, "poll")
	if !streamer.CompletedWatchStreak {
		t.Fatalf("fresh completion should keep propagation active when carryover is refreshed")
	}
	if !streamer.ResolvedStreakCarryoverUntil.After(firstCarryoverUntil) {
		t.Fatalf("fresh completion should refresh carryover expiry, got %s want after %s", streamer.ResolvedStreakCarryoverUntil, firstCarryoverUntil)
	}
}
