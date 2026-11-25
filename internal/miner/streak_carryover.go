package miner

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func resolvedStreakCarryoverExpiry(streamer *domain.Streamer, now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	base := now
	if streamer != nil && !streamer.OfflineAt.IsZero() && !streamer.OfflineAt.After(now) && now.Sub(streamer.OfflineAt) <= resolvedStreakCarryoverWindow {
		base = streamer.OfflineAt
	}
	return base.Add(resolvedStreakCarryoverWindow)
}

func rememberResolvedStreakCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if streamer.CompletedWatchStreak || activeResolvedStreakCarryover(streamer, now) {
		activateResolvedStreakCarryover(streamer, now)
		return
	}
	streamer.ResolvedStreakCarryover = false
	streamer.ResolvedStreakCarryoverUntil = time.Time{}
}

func restoreResolvedStreakCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if !activeResolvedStreakCarryover(streamer, now) {
		streamer.ResolvedStreakCarryover = false
		streamer.ResolvedStreakCarryoverUntil = time.Time{}
		streamer.CompletedWatchStreak = false
		return
	}
	streamer.CompletedWatchStreak = true
	applyResolvedStreakCarryover(streamer, now)
}

func activeResolvedStreakCarryover(streamer *domain.Streamer, now time.Time) bool {
	if streamer == nil || !streamer.ResolvedStreakCarryover || streamer.ResolvedStreakCarryoverUntil.IsZero() {
		return false
	}
	if now.After(streamer.ResolvedStreakCarryoverUntil) {
		streamer.ResolvedStreakCarryover = false
		streamer.ResolvedStreakCarryoverUntil = time.Time{}
		return false
	}
	return true
}

func applyResolvedStreakCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil || streamer.Stream == nil || !activeResolvedStreakCarryover(streamer, now) {
		return
	}
	streamer.Stream.WatchStreakMissing = false
}

func activateResolvedStreakCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	streamer.ResolvedStreakCarryover = true
	streamer.ResolvedStreakCarryoverUntil = resolvedStreakCarryoverExpiry(streamer, now)
}

func markActualStreakCompleted(streamer *domain.Streamer) {
	if streamer == nil {
		return
	}
	streamer.CompletedWatchStreak = true
	if streamer.Stream != nil {
		streamer.Stream.WatchStreakMissing = false
	}
}

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
		activateResolvedStreakCarryover(streamer, now)
	}
	applyResolvedStreakCarryover(streamer, now)
}
