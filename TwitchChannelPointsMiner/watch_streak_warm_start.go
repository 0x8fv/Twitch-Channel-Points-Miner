package twitchchannelpointsminer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

const (
	watchStreakWarmStartCacheVersion = 1
	watchStreakWarmStartFreshness    = 6 * time.Hour
)

type watchStreakWarmStartEntry struct {
	AccountName        string    `json:"account_name"`
	StreamerLogin      string    `json:"streamer_login"`
	ChannelID          string    `json:"channel_id,omitempty"`
	BroadcastID        string    `json:"broadcast_id,omitempty"`
	StreamCreatedAt    time.Time `json:"stream_created_at,omitempty"`
	WatchStreakMissing bool      `json:"watch_streak_missing"`
	CheckedAt          time.Time `json:"checked_at,omitempty"`
	IsOnline           bool      `json:"is_online"`
}

type watchStreakWarmStartFile struct {
	Version int                         `json:"version"`
	Entries []watchStreakWarmStartEntry `json:"entries"`
}

type watchStreakWarmStartCache struct {
	accountName string
	path        string
	mu          sync.Mutex
	entries     map[string]watchStreakWarmStartEntry
	dirty       bool
}

func newWatchStreakWarmStartCache(accountName, path string) *watchStreakWarmStartCache {
	return &watchStreakWarmStartCache{
		accountName: strings.ToLower(strings.TrimSpace(accountName)),
		path:        path,
		entries:     make(map[string]watchStreakWarmStartEntry),
	}
}

func loadWatchStreakWarmStartCache(path, accountName string) *watchStreakWarmStartCache {
	cache := newWatchStreakWarmStartCache(accountName, path)
	if path == "" {
		return cache
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return cache
	}

	var payload watchStreakWarmStartFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cache
	}
	if payload.Version != 0 && payload.Version != watchStreakWarmStartCacheVersion {
		return cache
	}

	accountKey := strings.ToLower(strings.TrimSpace(accountName))
	for _, entry := range payload.Entries {
		loginKey := strings.ToLower(strings.TrimSpace(entry.StreamerLogin))
		if loginKey == "" {
			continue
		}
		if accountKey != "" && strings.ToLower(strings.TrimSpace(entry.AccountName)) != accountKey {
			continue
		}
		cache.entries[loginKey] = entry
	}
	return cache
}

func (c *watchStreakWarmStartCache) key(streamerLogin string) string {
	return strings.ToLower(strings.TrimSpace(streamerLogin))
}

func sameWarmStartStream(entry watchStreakWarmStartEntry, streamer *entities.Streamer) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	broadcastID := strings.TrimSpace(streamer.Stream.BroadcastID)
	entryBroadcastID := strings.TrimSpace(entry.BroadcastID)
	if entryBroadcastID != "" && broadcastID != "" {
		return entryBroadcastID == broadcastID
	}
	if !entry.StreamCreatedAt.IsZero() && !streamer.Stream.CreatedAt.IsZero() {
		return entry.StreamCreatedAt.Equal(streamer.Stream.CreatedAt)
	}
	return false
}

func (c *watchStreakWarmStartCache) resolvedEntryForStreamer(streamer *entities.Streamer, now time.Time) (watchStreakWarmStartEntry, bool) {
	if c == nil || streamer == nil {
		return watchStreakWarmStartEntry{}, false
	}

	c.mu.Lock()
	entry, ok := c.entries[c.key(streamer.Username)]
	c.mu.Unlock()
	if !ok {
		return watchStreakWarmStartEntry{}, false
	}
	if !entry.IsOnline || entry.WatchStreakMissing {
		return watchStreakWarmStartEntry{}, false
	}
	if entry.CheckedAt.IsZero() || now.Sub(entry.CheckedAt) > watchStreakWarmStartFreshness {
		return watchStreakWarmStartEntry{}, false
	}
	if !sameWarmStartStream(entry, streamer) {
		return watchStreakWarmStartEntry{}, false
	}
	return entry, true
}

func (c *watchStreakWarmStartCache) get(streamerLogin string) (watchStreakWarmStartEntry, bool) {
	if c == nil {
		return watchStreakWarmStartEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[c.key(streamerLogin)]
	return entry, ok
}

func (c *watchStreakWarmStartCache) updateFromStreamer(streamer *entities.Streamer, checkedAt time.Time) {
	if c == nil || streamer == nil {
		return
	}
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}

	entry := watchStreakWarmStartEntry{
		AccountName:        strings.ToLower(strings.TrimSpace(c.accountName)),
		StreamerLogin:      strings.ToLower(strings.TrimSpace(streamer.Username)),
		ChannelID:          strings.TrimSpace(streamer.ChannelID),
		CheckedAt:          checkedAt,
		IsOnline:           streamer.IsOnline,
		WatchStreakMissing: true,
	}
	if streamer.Stream != nil {
		entry.BroadcastID = strings.TrimSpace(streamer.Stream.BroadcastID)
		entry.StreamCreatedAt = streamer.Stream.CreatedAt
		entry.WatchStreakMissing = streamer.Stream.WatchStreakMissing
	}

	key := c.key(streamer.Username)
	c.mu.Lock()
	current, exists := c.entries[key]
	if !exists || current != entry {
		c.entries[key] = entry
		c.dirty = true
	}
	c.mu.Unlock()
}

func (c *watchStreakWarmStartCache) saveIfDirty() error {
	if c == nil || c.path == "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	entries := make([]watchStreakWarmStartEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StreamerLogin < entries[j].StreamerLogin
	})

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	if err := utils.SaveJSON(c.path, watchStreakWarmStartFile{
		Version: watchStreakWarmStartCacheVersion,
		Entries: entries,
	}); err != nil {
		return err
	}

	c.dirty = false
	return nil
}
