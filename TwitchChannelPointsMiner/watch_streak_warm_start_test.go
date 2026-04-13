package twitchchannelpointsminer

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func testWarmStartStreamer(login, channelID, broadcastID string, createdAt time.Time, online bool) *entities.Streamer {
	stream := entities.NewStream()
	stream.BroadcastID = broadcastID
	stream.CreatedAt = createdAt
	stream.WatchStreakMissing = true
	return &entities.Streamer{
		Username:      login,
		ChannelID:     channelID,
		IsOnline:      online,
		PresenceKnown: true,
		Stream:        stream,
	}
}

func discardLogger() *Logger {
	return &Logger{base: log.New(io.Discard, "", 0)}
}

func TestWatchStreakWarmStartCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	checkedAt := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	createdAt := checkedAt.Add(-20 * time.Minute)
	cache := newWatchStreakWarmStartCache("Account", path)
	streamer := testWarmStartStreamer("Streamer", "123", "broadcast-1", createdAt, true)
	streamer.Stream.WatchStreakMissing = false

	cache.updateFromStreamer(streamer, checkedAt)
	if err := cache.saveIfDirty(); err != nil {
		t.Fatalf("saveIfDirty returned error: %v", err)
	}

	loaded := loadWatchStreakWarmStartCache(path, "account")
	entry, ok := loaded.get("streamer")
	if !ok {
		t.Fatalf("expected saved entry to load")
	}
	if entry.AccountName != "account" {
		t.Fatalf("account name got %q want %q", entry.AccountName, "account")
	}
	if entry.StreamerLogin != "streamer" {
		t.Fatalf("streamer login got %q want %q", entry.StreamerLogin, "streamer")
	}
	if entry.ChannelID != "123" || entry.BroadcastID != "broadcast-1" {
		t.Fatalf("unexpected identifiers in entry: %#v", entry)
	}
	if entry.WatchStreakMissing {
		t.Fatalf("expected resolved watch streak to persist")
	}
	if !entry.CheckedAt.Equal(checkedAt) {
		t.Fatalf("checkedAt got %s want %s", entry.CheckedAt, checkedAt)
	}
	if !entry.StreamCreatedAt.Equal(createdAt) {
		t.Fatalf("createdAt got %s want %s", entry.StreamCreatedAt, createdAt)
	}
}

func TestLoadWatchStreakWarmStartCacheHandlesMissingAndMalformedFiles(t *testing.T) {
	missing := loadWatchStreakWarmStartCache(filepath.Join(t.TempDir(), "missing.json"), "account")
	if _, ok := missing.get("streamer"); ok {
		t.Fatalf("missing cache file should start empty")
	}

	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed cache: %v", err)
	}
	malformed := loadWatchStreakWarmStartCache(path, "account")
	if _, ok := malformed.get("streamer"); ok {
		t.Fatalf("malformed cache file should fail safe and start empty")
	}
}

func TestWatchStreakWarmStartMatchingRules(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-30 * time.Minute)
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)

	cache := newWatchStreakWarmStartCache("account", "")
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		BroadcastID:        "broadcast-1",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); !ok {
		t.Fatalf("matching broadcast id should be trusted")
	}

	streamer.Stream.BroadcastID = ""
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); !ok {
		t.Fatalf("matching createdAt should be trusted when broadcast id is unavailable")
	}

	streamer.Stream.BroadcastID = "broadcast-2"
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		BroadcastID:        "broadcast-1",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); ok {
		t.Fatalf("changed broadcast id should not be trusted")
	}

	streamer.Stream.BroadcastID = ""
	streamer.Stream.CreatedAt = createdAt.Add(time.Minute)
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); ok {
		t.Fatalf("changed createdAt should not be trusted")
	}

	streamer.Stream.CreatedAt = createdAt
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-watchStreakWarmStartFreshness).Add(-time.Minute),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); ok {
		t.Fatalf("stale entry should not be trusted")
	}

	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           false,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); ok {
		t.Fatalf("offline cache entry should not be trusted")
	}

	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: true,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.resolvedEntryForStreamer(streamer, now); ok {
		t.Fatalf("pending cache entry should not be trusted")
	}
}

func TestApplyWarmStartCacheMarksResolvedStreamComplete(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)
	cache := newWatchStreakWarmStartCache("account", "")
	cache.entries["streamer"] = watchStreakWarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		BroadcastID:        "broadcast-1",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now,
		IsOnline:           true,
	}

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
		cache := newWatchStreakWarmStartCache("account", path)
		m := &Miner{
			logger:         discardLogger(),
			warmStartCache: cache,
		}
		streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)
		streamer.ChannelPoints = 100
		streamer.PointsInit = true

		m.handlePubSubGain(streamer, 10, "WATCH", 0)
		if _, ok := cache.get("streamer"); ok {
			t.Fatalf("first WATCH should not persist unresolved streak state")
		}

		m.handlePubSubGain(streamer, 10, "WATCH", 0)
		entry, ok := cache.get("streamer")
		if !ok {
			t.Fatalf("second WATCH should persist resolved streak state")
		}
		if entry.WatchStreakMissing {
			t.Fatalf("second WATCH should persist a resolved streak state")
		}
	})

	t.Run("watch streak", func(t *testing.T) {
		cache := newWatchStreakWarmStartCache("account", path)
		m := &Miner{
			logger:         discardLogger(),
			warmStartCache: cache,
		}
		streamer := testWarmStartStreamer("streamer2", "456", "broadcast-2", createdAt, true)
		streamer.ChannelPoints = 100
		streamer.PointsInit = true

		m.handlePubSubGain(streamer, 450, "WATCH_STREAK", 0)
		entry, ok := cache.get("streamer2")
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
	cache := newWatchStreakWarmStartCache("account", path)
	m := &Miner{
		logger:         discardLogger(),
		warmStartCache: cache,
	}
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC), true)

	m.setPresence(streamer, false, "poll")

	entry, ok := cache.get("streamer")
	if !ok {
		t.Fatalf("offline presence should be persisted")
	}
	if entry.IsOnline {
		t.Fatalf("offline presence should invalidate the cached online snapshot")
	}
}

func TestPickStreamersToWatchPersistsTimedOutStreakToWarmStartCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	cache := newWatchStreakWarmStartCache("account", path)
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
	streamer.Stream.MinuteWatched = streakPriorityMinutesBase

	m := &Miner{
		logger:         discardLogger(),
		warmStartCache: cache,
	}

	m.pickStreamersToWatch([]*entities.Streamer{streamer})

	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("timed-out streak should be marked resolved during watch selection")
	}
	entry, ok := cache.get("streamer")
	if !ok {
		t.Fatalf("timed-out streak should be written to cache")
	}
	if entry.WatchStreakMissing {
		t.Fatalf("cache should persist timed-out streak as resolved")
	}
}
