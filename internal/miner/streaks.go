package miner

import (
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/streak"
)

func (m *Miner) streakRecovery(streamers []*domain.Streamer, stop <-chan struct{}) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()
	for {
		expiringstreaks := 0
		savedstreaks := 0
		for _, s := range streamers {
			if saved, err := m.twitch.RecoverStreak(s); err == nil {
				if saved {
					expiringstreaks += 1
					savedstreaks += 1
				}
			} else {
				expiringstreaks += 1
				m.logger.EmojiPrintf(":ambulance:", "streak recovery %s: %v", m.styledStreamerName(s), err)
			}
		}
		m.logger.EmojiPrintf(":ambulance:", "streak recovery: %d streaks expiring, %d saved", expiringstreaks, savedstreaks)

		select {
		case <-ticker.C:
			continue
		case <-stop:
			return
		}
	}
}

func (m *Miner) resolveTimedOutStreak(streamer *domain.Streamer, now time.Time) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if !streamer.Settings.WatchStreak || !streamer.Stream.WatchStreakMissing {
		return false
	}
	if streamer.Stream.MinuteWatched < m.streakPriorityLimit(now) {
		return false
	}
	streamer.Stream.WatchStreakMissing = false
	return true
}

func (m *Miner) streakCooldownBlocksCurrentStream(streamer *domain.Streamer, now time.Time) bool {
	if streamer == nil || streamer.OfflineAt.IsZero() {
		return false
	}
	if now.Sub(streamer.OfflineAt) > 30*time.Minute {
		return false
	}
	if streamer.Stream == nil {
		return true
	}
	if streak.StreamStartSurvivesOffline(streamer.Stream.StreamUpAt, streamer.OfflineAt) {
		return false
	}
	if streak.StreamStartSurvivesOffline(streamer.Stream.CreatedAt, streamer.OfflineAt) {
		return false
	}
	return true
}

func (m *Miner) shouldPrioritizeStreak(streamer *domain.Streamer, now time.Time) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if !streamer.Settings.WatchStreak || streak.CarryoverActive(streamer, now) || !streamer.Stream.WatchStreakMissing {
		return false
	}
	if m.streakCooldownBlocksCurrentStream(streamer, now) {
		return false
	}
	// ? Keep streak priority long enough for Twitch to issue the streak check (typically ~15 minutes).
	return streamer.Stream.MinuteWatched < m.streakPriorityLimit(now)
}

func (m *Miner) activeStreakWatchKey(streamer *domain.Streamer) string {
	if streamer == nil {
		return ""
	}
	if channelID := strings.TrimSpace(streamer.ChannelID); channelID != "" {
		return channelID
	}
	return strings.ToLower(strings.TrimSpace(streamer.Username))
}

func (m *Miner) shouldKeepActiveStreak(streamer *domain.Streamer, now time.Time) bool {
	if !m.shouldPrioritizeStreak(streamer, now) {
		return false
	}
	return streamer.Stream.MinuteWatched > 0
}

func (m *Miner) syncActiveStreakWatch(streamer *domain.Streamer, now time.Time) {
	if m == nil {
		return
	}
	key := m.activeStreakWatchKey(streamer)
	if key == "" {
		return
	}
	if !m.shouldKeepActiveStreak(streamer, now) {
		m.activeStreakMu.Lock()
		if m.activeStreakWatches != nil {
			delete(m.activeStreakWatches, key)
		}
		m.activeStreakMu.Unlock()
		return
	}
	m.activeStreakMu.Lock()
	if m.activeStreakWatches == nil {
		m.activeStreakWatches = make(map[string]streak.ActiveWatch)
	}
	m.activeStreakWatches[key] = streak.ActiveWatch{
		BroadcastID: streamer.Stream.BroadcastID,
		CreatedAt:   streamer.Stream.CreatedAt,
	}
	m.activeStreakMu.Unlock()
}

// ? streakPriorityLimit adjusts streak priority duration:
// ? - default 7 minutes
// ? - extended to 20 minutes after 10 hours runtime to avoid churn late in long sessions.
func (m *Miner) streakPriorityLimit(now time.Time) float64 {
	if m == nil {
		return streak.PriorityMinutesBase
	}
	if m.startedAt.IsZero() {
		return streak.PriorityMinutesBase
	}
	if now.Sub(m.startedAt) > 10*time.Hour {
		return streak.PriorityMinutesExtended
	}
	return streak.PriorityMinutesBase
}
