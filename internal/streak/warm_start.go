package streak

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/jsonfile"
)

const (
	warmStartCacheVersion = 1
	WarmStartFreshness    = 6 * time.Hour
)

type WarmStartEntry struct {
	AccountName        string    `json:"account_name"`
	StreamerLogin      string    `json:"streamer_login"`
	ChannelID          string    `json:"channel_id,omitempty"`
	BroadcastID        string    `json:"broadcast_id,omitempty"`
	StreamCreatedAt    time.Time `json:"stream_created_at,omitempty"`
	WatchStreakMissing bool      `json:"watch_streak_missing"`
	CheckedAt          time.Time `json:"checked_at,omitempty"`
	IsOnline           bool      `json:"is_online"`
}

type warmStartFile struct {
	Version int              `json:"version"`
	Entries []WarmStartEntry `json:"entries"`
}

type WarmStartCache struct {
	accountName string
	path        string
	mu          sync.Mutex
	entries     map[string]WarmStartEntry
	dirty       bool
}

func NewWarmStartCache(accountName, path string) *WarmStartCache {
	return &WarmStartCache{
		accountName: strings.ToLower(strings.TrimSpace(accountName)),
		path:        path,
		entries:     make(map[string]WarmStartEntry),
	}
}

func LoadWarmStartCache(path, accountName string) *WarmStartCache {
	cache := NewWarmStartCache(accountName, path)
	if path == "" {
		return cache
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return cache
	}

	var payload warmStartFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cache
	}
	if payload.Version != 0 && payload.Version != warmStartCacheVersion {
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

func (c *WarmStartCache) key(streamerLogin string) string {
	return strings.ToLower(strings.TrimSpace(streamerLogin))
}

func SameWarmStartStream(entry WarmStartEntry, streamer *domain.Streamer) bool {
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

func (c *WarmStartCache) ResolvedEntryFor(streamer *domain.Streamer, now time.Time) (WarmStartEntry, bool) {
	if c == nil || streamer == nil {
		return WarmStartEntry{}, false
	}

	c.mu.Lock()
	entry, ok := c.entries[c.key(streamer.Username)]
	c.mu.Unlock()
	if !ok {
		return WarmStartEntry{}, false
	}
	if !entry.IsOnline || entry.WatchStreakMissing {
		return WarmStartEntry{}, false
	}
	if entry.CheckedAt.IsZero() || now.Sub(entry.CheckedAt) > WarmStartFreshness {
		return WarmStartEntry{}, false
	}
	if !SameWarmStartStream(entry, streamer) {
		return WarmStartEntry{}, false
	}
	return entry, true
}

func (c *WarmStartCache) Get(streamerLogin string) (WarmStartEntry, bool) {
	if c == nil {
		return WarmStartEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[c.key(streamerLogin)]
	return entry, ok
}

func (c *WarmStartCache) UpdateFromStreamer(streamer *domain.Streamer, checkedAt time.Time) {
	if c == nil || streamer == nil {
		return
	}
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}

	entry := WarmStartEntry{
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

func (c *WarmStartCache) SaveIfDirty() error {
	if c == nil || c.path == "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	entries := make([]WarmStartEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StreamerLogin < entries[j].StreamerLogin
	})

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	if err := jsonfile.SaveJSON(c.path, warmStartFile{
		Version: warmStartCacheVersion,
		Entries: entries,
	}); err != nil {
		return err
	}

	c.dirty = false
	return nil
}
