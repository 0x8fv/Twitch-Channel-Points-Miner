package miner

import (
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

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
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
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

	streamer.OfflineAt = time.Time{}
	streamer.Stream.WatchStreakMissing = false
	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("resolved streak should not keep streak priority")
	}
}

func TestShouldPrioritizeStreakAllowsRestartedStreamWithinOfflineCooldown(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)

	m := &Miner{}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         now.Add(-9 * time.Minute),
			CreatedAt:          now.Add(-9 * time.Minute),
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("restarted stream should regain streak priority inside the offline cooldown")
	}
}

func TestShouldPrioritizeStreakAllowsStreamStartedJustBeforeFalseOffline(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)
	streamStartedAt := offlineAt.Add(-20 * time.Second)

	m := &Miner{}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("stream started just before a false offline event should regain streak priority")
	}
}

func TestShouldPrioritizeStreakBlocksStreamStartedWellBeforeRecentOffline(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)
	streamStartedAt := offlineAt.Add(-5 * time.Minute)

	m := &Miner{}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("stream started well before a recent offline event should stay blocked by cooldown")
	}
}

func TestResolveTimedOutStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      streakPriorityMinutesBase,
		},
	}

	if !m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("expected timed-out streak to resolve")
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("timed-out streak should clear pending state")
	}
	if m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("resolved streak should not be resolved twice")
	}
}

func TestResolveTimedOutStreakShortRestartStillPrioritizesStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &domain.Streamer{
		Username:      "timedout",
		ChannelID:     "channel-timedout",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      streakPriorityMinutesBase,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	if !m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("expected timed-out streak to resolve before restart")
	}

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if !m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("timed-out short restart should still prioritize streak")
	}
}

func TestSyncActiveStreakWatchTracksOnlyEligibleInProgressStreaks(t *testing.T) {
	now := time.Now()
	streamer := testWatchSelectionStreakStreamer("first", "channel-first", "casual game", now.Add(-time.Minute), 0)
	m := &Miner{}

	m.syncActiveStreakWatch(streamer, now)
	if _, ok := m.activeStreakWatches[streamer.ChannelID]; ok {
		t.Fatalf("fresh streak without progress should not be tracked")
	}

	streamer.Stream.MinuteWatched = 1.25
	m.syncActiveStreakWatch(streamer, now)
	watch, ok := m.activeStreakWatches[streamer.ChannelID]
	if !ok {
		t.Fatalf("eligible streak should be tracked")
	}
	if watch.BroadcastID != streamer.Stream.BroadcastID {
		t.Fatalf("tracked broadcast mismatch got %q want %q", watch.BroadcastID, streamer.Stream.BroadcastID)
	}

	streamer.Stream.WatchStreakMissing = false
	m.syncActiveStreakWatch(streamer, now)
	if _, ok := m.activeStreakWatches[streamer.ChannelID]; ok {
		t.Fatalf("resolved streak should be removed from active tracking")
	}
}
