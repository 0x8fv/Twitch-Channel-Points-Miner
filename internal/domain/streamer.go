package domain

import (
	"sync"
	"time"
)

type Streamer struct {
	Username                     string             `json:"username"`
	ChannelID                    string             `json:"channel_id"`
	ChannelPoints                int                `json:"channel_points"`
	Settings                     StreamerSettings   `json:"settings"`
	StreamerURL                  string             `json:"-"`
	IsOnline                     bool               `json:"-"`
	PresenceKnown                bool               `json:"-"`
	OnlineAt                     time.Time          `json:"-"`
	OfflineAt                    time.Time          `json:"-"`
	CompletedWatchStreak         bool               `json:"-"`
	ResolvedStreakCarryover      bool               `json:"-"`
	ResolvedStreakCarryoverUntil time.Time          `json:"-"`
	WatchFailureCount            int                `json:"-"`
	WatchBackoffUntil            time.Time          `json:"-"`
	Stream                       *Stream            `json:"-"`
	PointsInit                   bool               `json:"-"`
	ActiveMultipliers            []ActiveMultiplier `json:"-"`
	LastRaidID                   string             `json:"-"`
	History                      map[string]*HistoryEntry
	HistoryMu                    sync.Mutex
	CommunityGoals               map[string]*CommunityGoal `json:"-"`
}

type ActiveMultiplier struct {
	Factor float64 `json:"factor"`
}

type HistoryEntry struct {
	Count  int
	Amount int
}

func (s *Streamer) HasActiveMultipliers() bool {
	return len(s.ActiveMultipliers) > 0
}

func (s *Streamer) TotalMultiplier() float64 {
	total := 0.0
	for _, mult := range s.ActiveMultipliers {
		total += mult.Factor
	}
	return total
}

func (s *Streamer) PredictionWindowSeconds(predictionWindow float64) float64 {
	delay := 0.0
	if s.Settings.Bet.Delay != nil {
		delay = *s.Settings.Bet.Delay
	}
	switch s.Settings.Bet.DelayMode {
	case DelayModeFromStart:
		if delay < predictionWindow {
			return delay
		}
		return predictionWindow
	case DelayModeFromEnd:
		if predictionWindow-delay > 0 {
			return predictionWindow - delay
		}
		return 0
	case DelayModePercentage:
		return predictionWindow * delay
	default:
		return predictionWindow
	}
}
