package streak

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

const (
	PriorityMinutesBase     = 7.0
	PriorityMinutesExtended = 20.0
	CarryoverWindow         = 30 * time.Minute
	FalseOfflineStartGrace  = 2 * time.Minute
)

type ActiveWatch struct {
	BroadcastID string
	CreatedAt   time.Time
}

func ActiveWatchMatches(streamer *domain.Streamer, watch ActiveWatch) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if streamer.Stream.BroadcastID != "" || watch.BroadcastID != "" {
		return streamer.Stream.BroadcastID != "" && streamer.Stream.BroadcastID == watch.BroadcastID
	}
	if streamer.Stream.CreatedAt.IsZero() || watch.CreatedAt.IsZero() {
		return false
	}
	return streamer.Stream.CreatedAt.Equal(watch.CreatedAt)
}

func StreamStartSurvivesOffline(start, offlineAt time.Time) bool {
	if start.IsZero() || offlineAt.IsZero() {
		return false
	}
	if !start.Before(offlineAt) {
		return true
	}
	return offlineAt.Sub(start) <= FalseOfflineStartGrace
}
