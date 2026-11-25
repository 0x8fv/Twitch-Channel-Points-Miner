package miner

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/logging"
	"TwitchChannelPointsMiner/internal/streak"
)

func testWarmStartStreamer(login, channelID, broadcastID string, createdAt time.Time, online bool) *domain.Streamer {
	stream := domain.NewStream()
	stream.BroadcastID = broadcastID
	stream.CreatedAt = createdAt
	stream.WatchStreakMissing = true
	return &domain.Streamer{
		Username:      login,
		ChannelID:     channelID,
		IsOnline:      online,
		PresenceKnown: true,
		Stream:        stream,
	}
}

func discardLogger() *logging.Logger {
	return logging.NewLoggerTo(io.Discard, logging.LoggerSettings{})
}

func TestApplyWarmStartCacheMarksResolvedStreamComplete(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)
	cache := streak.NewWarmStartCache("account", "")
	// The cache holds the same stream with its streak already resolved.
	resolved := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)
	resolved.Stream.WatchStreakMissing = false
	cache.UpdateFromStreamer(resolved, now)

	m := &Miner{warmStartCache: cache}
	if !m.applyWarmStartCache(streamer) {
		t.Fatalf("expected warm-start cache to apply")
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("warm-start cache should resolve pending streak")
	}
}

func TestHandlePubSubGainPersistsResolvedWatchStreakState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	createdAt := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)

	t.Run("watch fallback", func(t *testing.T) {
		cache := streak.NewWarmStartCache("account", path)
		m := &Miner{
			logger:         discardLogger(),
			warmStartCache: cache,
		}
		streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)
		streamer.ChannelPoints = 100
		streamer.PointsInit = true

		m.handlePubSubGain(streamer, 10, "WATCH", 0)
		if _, ok := cache.Get("streamer"); ok {
			t.Fatalf("first WATCH should not persist unresolved streak state")
		}

		m.handlePubSubGain(streamer, 10, "WATCH", 0)
		entry, ok := cache.Get("streamer")
		if !ok {
			t.Fatalf("second WATCH should persist resolved streak state")
		}
		if entry.WatchStreakMissing {
			t.Fatalf("second WATCH should persist a resolved streak state")
		}
	})

	t.Run("watch streak", func(t *testing.T) {
		cache := streak.NewWarmStartCache("account", path)
		m := &Miner{
			logger:         discardLogger(),
			warmStartCache: cache,
		}
		streamer := testWarmStartStreamer("streamer2", "456", "broadcast-2", createdAt, true)
		streamer.ChannelPoints = 100
		streamer.PointsInit = true

		m.handlePubSubGain(streamer, 450, "WATCH_STREAK", 0)
		entry, ok := cache.Get("streamer2")
		if !ok {
			t.Fatalf("WATCH_STREAK should persist resolved streak state")
		}
		if entry.WatchStreakMissing {
			t.Fatalf("WATCH_STREAK should persist a resolved streak state")
		}
	})
}

func TestSetPresenceInvalidatesWarmStartCacheWhenStreamerGoesOffline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	cache := streak.NewWarmStartCache("account", path)
	m := &Miner{
		logger:         discardLogger(),
		warmStartCache: cache,
	}
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC), true)

	m.setPresence(streamer, false, "poll")

	entry, ok := cache.Get("streamer")
	if !ok {
		t.Fatalf("offline presence should be persisted")
	}
	if entry.IsOnline {
		t.Fatalf("offline presence should invalidate the cached online snapshot")
	}
}

func TestPickStreamersToWatchPersistsTimedOutStreakToWarmStartCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	cache := streak.NewWarmStartCache("account", path)
	now := time.Now()
	streamer := testWarmStartStreamer(
		"streamer",
		"123",
		"broadcast-1",
		now.Add(-30*time.Minute),
		true,
	)
	streamer.Settings.WatchStreak = true
	streamer.OnlineAt = now.Add(-time.Minute)
	streamer.Stream.MinuteWatched = streak.PriorityMinutesBase

	m := &Miner{
		logger:         discardLogger(),
		warmStartCache: cache,
	}

	m.pickStreamersToWatch([]*domain.Streamer{streamer})

	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("timed-out streak should be marked resolved during watch selection")
	}
	entry, ok := cache.Get("streamer")
	if !ok {
		t.Fatalf("timed-out streak should be written to cache")
	}
	if entry.WatchStreakMissing {
		t.Fatalf("cache should persist timed-out streak as resolved")
	}
}
