package streak

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func CarryoverExpiry(streamer *domain.Streamer, now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	base := now
	if streamer != nil && !streamer.OfflineAt.IsZero() && !streamer.OfflineAt.After(now) && now.Sub(streamer.OfflineAt) <= CarryoverWindow {
		base = streamer.OfflineAt
	}
	return base.Add(CarryoverWindow)
}

func RememberCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if streamer.CompletedWatchStreak || CarryoverActive(streamer, now) {
		ActivateCarryover(streamer, now)
		return
	}
	streamer.ResolvedStreakCarryover = false
	streamer.ResolvedStreakCarryoverUntil = time.Time{}
}

func RestoreCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if !CarryoverActive(streamer, now) {
		streamer.ResolvedStreakCarryover = false
		streamer.ResolvedStreakCarryoverUntil = time.Time{}
		streamer.CompletedWatchStreak = false
		return
	}
	streamer.CompletedWatchStreak = true
	ApplyCarryover(streamer, now)
}

func CarryoverActive(streamer *domain.Streamer, now time.Time) bool {
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

func ApplyCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil || streamer.Stream == nil || !CarryoverActive(streamer, now) {
		return
	}
	streamer.Stream.WatchStreakMissing = false
}

func ActivateCarryover(streamer *domain.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	streamer.ResolvedStreakCarryover = true
	streamer.ResolvedStreakCarryoverUntil = CarryoverExpiry(streamer, now)
}

func MarkCompleted(streamer *domain.Streamer) {
	if streamer == nil {
		return
	}
	streamer.CompletedWatchStreak = true
	if streamer.Stream != nil {
		streamer.Stream.WatchStreakMissing = false
	}
}
