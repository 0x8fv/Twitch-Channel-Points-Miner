package miner

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/streak"
)

func (m *Miner) syncResolvedStreakState(streamer *domain.Streamer, prevBroadcastID string, prevCreatedAt time.Time, prevWatchStreakMissing bool) {
	if streamer == nil || streamer.Stream == nil {
		return
	}
	if prevWatchStreakMissing && !streamer.Stream.WatchStreakMissing {
		streamer.CompletedWatchStreak = true
	}
	segmentRolled := (prevBroadcastID != "" && streamer.Stream.BroadcastID != "" && prevBroadcastID != streamer.Stream.BroadcastID) ||
		(!prevCreatedAt.IsZero() && !streamer.Stream.CreatedAt.IsZero() && !prevCreatedAt.Equal(streamer.Stream.CreatedAt))
	now := time.Now()
	if segmentRolled && streamer.CompletedWatchStreak {
		streak.ActivateCarryover(streamer, now)
	}
	streak.ApplyCarryover(streamer, now)
}
