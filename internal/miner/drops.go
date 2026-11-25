package miner

import (
	"TwitchChannelPointsMiner/internal/domain"
)

func (m *Miner) shouldClaimDrops(streamers []*domain.Streamer) bool {
	if m.StreamerSettings.ClaimDrops {
		return true
	}
	for _, streamer := range streamers {
		if streamer != nil && streamer.Settings.ClaimDrops {
			return true
		}
	}
	return false
}
