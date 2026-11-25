package streak

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
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

func TestWatchStreakWarmStartCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch_streak_cache.account.json")
	checkedAt := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	createdAt := checkedAt.Add(-20 * time.Minute)
	cache := NewWarmStartCache("Account", path)
	streamer := testWarmStartStreamer("Streamer", "123", "broadcast-1", createdAt, true)
	streamer.Stream.WatchStreakMissing = false

	cache.UpdateFromStreamer(streamer, checkedAt)
	if err := cache.SaveIfDirty(); err != nil {
		t.Fatalf("saveIfDirty returned error: %v", err)
	}

	loaded := LoadWarmStartCache(path, "account")
	entry, ok := loaded.Get("streamer")
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
	missing := LoadWarmStartCache(filepath.Join(t.TempDir(), "missing.json"), "account")
	if _, ok := missing.Get("streamer"); ok {
		t.Fatalf("missing cache file should start empty")
	}

	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed cache: %v", err)
	}
	malformed := LoadWarmStartCache(path, "account")
	if _, ok := malformed.Get("streamer"); ok {
		t.Fatalf("malformed cache file should fail safe and start empty")
	}
}

func TestWatchStreakWarmStartMatchingRules(t *testing.T) {
	now := time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-30 * time.Minute)
	streamer := testWarmStartStreamer("streamer", "123", "broadcast-1", createdAt, true)

	cache := NewWarmStartCache("account", "")
	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		BroadcastID:        "broadcast-1",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); !ok {
		t.Fatalf("matching broadcast id should be trusted")
	}

	streamer.Stream.BroadcastID = ""
	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); !ok {
		t.Fatalf("matching createdAt should be trusted when broadcast id is unavailable")
	}

	streamer.Stream.BroadcastID = "broadcast-2"
	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		BroadcastID:        "broadcast-1",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); ok {
		t.Fatalf("changed broadcast id should not be trusted")
	}

	streamer.Stream.BroadcastID = ""
	streamer.Stream.CreatedAt = createdAt.Add(time.Minute)
	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); ok {
		t.Fatalf("changed createdAt should not be trusted")
	}

	streamer.Stream.CreatedAt = createdAt
	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-WarmStartFreshness).Add(-time.Minute),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); ok {
		t.Fatalf("stale entry should not be trusted")
	}

	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: false,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           false,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); ok {
		t.Fatalf("offline cache entry should not be trusted")
	}

	cache.entries["streamer"] = WarmStartEntry{
		AccountName:        "account",
		StreamerLogin:      "streamer",
		ChannelID:          "123",
		StreamCreatedAt:    createdAt,
		WatchStreakMissing: true,
		CheckedAt:          now.Add(-time.Hour),
		IsOnline:           true,
	}
	if _, ok := cache.ResolvedEntryFor(streamer, now); ok {
		t.Fatalf("pending cache entry should not be trusted")
	}
}
