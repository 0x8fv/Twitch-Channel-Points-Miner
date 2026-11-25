package miner

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func (m *Miner) loadWarmStartCache() {
	if m == nil || !m.WatchStreakWarmStartCache {
		return
	}
	m.warmStartCache = loadWatchStreakWarmStartCache(m.warmStartCachePath, m.Username)
}

func (m *Miner) saveWarmStartCacheSnapshot(streamers []*domain.Streamer) {
	if m == nil || m.warmStartCache == nil {
		return
	}
	now := time.Now()
	for _, streamer := range streamers {
		m.warmStartCache.updateFromStreamer(streamer, now)
	}
	if err := m.warmStartCache.saveIfDirty(); err != nil && m.logger != nil {
		m.logger.Debugf("warm-start cache save failed: %v", err)
	}
}

func (m *Miner) syncWarmStartCacheFromStreamer(streamer *domain.Streamer) {
	if m == nil || m.warmStartCache == nil || streamer == nil {
		return
	}
	m.warmStartCache.updateFromStreamer(streamer, time.Now())
	if err := m.warmStartCache.saveIfDirty(); err != nil && m.logger != nil {
		m.logger.Debugf("warm-start cache save failed for %s: %v", m.rawStreamerName(streamer.Username), err)
	}
}

func (m *Miner) applyWarmStartCache(streamer *domain.Streamer) bool {
	if m == nil || m.warmStartCache == nil || streamer == nil || streamer.Stream == nil || !streamer.IsOnline {
		return false
	}
	entry, ok := m.warmStartCache.resolvedEntryForStreamer(streamer, time.Now())
	if !ok {
		return false
	}
	streamer.Stream.WatchStreakMissing = entry.WatchStreakMissing
	return true
}
