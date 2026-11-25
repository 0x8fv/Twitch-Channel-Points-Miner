package miner

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func (m *Miner) resolveGameName(streamer *domain.Streamer) string {
	if streamer == nil {
		return ""
	}
	if streamer.Stream != nil {
		if name := strings.TrimSpace(streamer.Stream.GameName()); name != "" {
			return name
		}
	}
	if !m.showGameInfo || m.twitch == nil || !streamer.IsOnline {
		return ""
	}
	prevBroadcastID := ""
	prevCreatedAt := time.Time{}
	prevWatchStreakMissing := true
	if streamer.Stream != nil {
		prevBroadcastID = streamer.Stream.BroadcastID
		prevCreatedAt = streamer.Stream.CreatedAt
		prevWatchStreakMissing = streamer.Stream.WatchStreakMissing
	}
	if err := m.updateStreamWithIdentityRepair(streamer); err != nil {
		if m.logger != nil && m.logger.DebugEnabled() {
			m.logger.Debugf("update stream %s for game: %v", m.styledStreamerName(streamer), err)
		}
		return ""
	}
	m.syncResolvedStreakState(streamer, prevBroadcastID, prevCreatedAt, prevWatchStreakMissing)
	if streamer.Stream != nil && (prevBroadcastID != streamer.Stream.BroadcastID || !prevCreatedAt.Equal(streamer.Stream.CreatedAt) || prevWatchStreakMissing != streamer.Stream.WatchStreakMissing) {
		m.syncWarmStartCacheFromStreamer(streamer)
	}
	if streamer.Stream != nil {
		return strings.TrimSpace(streamer.Stream.GameName())
	}
	return ""
}

func (m *Miner) gameSuffix(streamer *domain.Streamer) string {
	name := m.gameInfo(streamer)
	if name != "" {
		return name
	}
	return ""
}

// func (m *Miner) gameInfo(streamer *domain.Streamer) (string, bool) {
func (m *Miner) gameInfo(streamer *domain.Streamer) string {
	if m == nil || !m.showGameInfo {
		return ""
	}
	// hasDrops := m.showDropsIndicator && m.streamHasDrops(streamer)
	// if !m.showGameInfo {
	// 	return "", hasDrops
	// }
	// return m.resolveGameName(streamer), hasDrops
	return m.resolveGameName(streamer)
}

func (m *Miner) watchContext(streamer *domain.Streamer) string {
	// name, hasDrops := m.gameInfo(streamer)
	name := m.gameInfo(streamer)
	if name != "" {
		label := fmt.Sprintf("| %sGame:%s", colorGameLabel, colorReset)
		// if hasDrops {
		// 	return fmt.Sprintf("%s %s %s", label, name, m.dropIndicator())
		// }
		return fmt.Sprintf("%s %s", label, name)
	}
	// if hasDrops {
	// 	return m.dropIndicator()
	// }
	return ""
}

// TODO: Fix Available Campaigns
// func (m *Miner) dropIndicator() string {
// 	if !m.showDropsIndicator {
// 		return ""
// 	}
// 	return fmt.Sprintf("%s(DROPS)%s", colorDropsAccent, colorReset)
// }

// func (m *Miner) streamHasDrops(streamer *domain.Streamer) bool {
// 	if streamer == nil || streamer.Stream == nil {
// 		return false
// 	}
// 	return m.twitch != nil && m.twitch.GameHasActiveDrops(streamer.Stream)
// }

// func (m *Miner) refreshCampaigns(streamer *domain.Streamer) {
// 	if streamer == nil || streamer.Stream == nil || m.twitch == nil {
// 		return
// 	}
// 	if !(streamer.Settings.ClaimDrops || m.showDropsIndicator) {
// 		return
// 	}
// 	campaigns, hasDrops, err := m.twitch.CampaignIDsForStreamer(streamer)
// 	if err != nil {
// 		if m.logger != nil && m.logger.DebugEnabled() {
// 			m.logger.Debugf("campaigns for %s: %v", streamer.Username, err)
// 		}
// 		return
// 	}
// 	streamer.Stream.CampaignIDs = campaigns
// 	streamer.Stream.CampaignsResolved = true
// 	streamer.Stream.DropsActive = hasDrops || m.twitch.GameHasActiveDrops(streamer.Stream)
// }

func (m *Miner) handleGameChange(streamer *domain.Streamer, previous, current string) {
	if m == nil || m.logger == nil || !m.showGameInfo {
		return
	}
	current = strings.TrimSpace(current)
	previous = strings.TrimSpace(previous)
	// ? Skip initial load while bringing a streamer online to avoid duplicate "now playing" spam.
	if current == "" || previous == "" || strings.EqualFold(previous, current) {
		return
	}
	m.logger.EmojiPrintf(":video_game:", "%s now playing: %s!", m.styledStreamerName(streamer), current)
}
