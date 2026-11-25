package miner

import (
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/twitch"
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

func (m *Miner) dropClaimer(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.claimDropsFromInventory("drop claim failed")
		case <-stop:
			return
		}
	}
}

func (m *Miner) claimDropsFromInventory(errorPrefix string) {
	if m == nil || m.twitch == nil {
		return
	}
	if m.showDropsProgress {
		drops, statuses, err := m.twitch.ClaimAllDropsFromInventoryWithStatuses()
		if err != nil {
			m.logger.Printf("%s: %v", errorPrefix, err)
			return
		}
		m.logDropStatuses(statuses)
		m.logClaimedDrops(drops)
		return
	}
	drops, err := m.twitch.ClaimAllDropsFromInventory()
	if err != nil {
		m.logger.Printf("%s: %v", errorPrefix, err)
		return
	}
	m.logClaimedDrops(drops)
}

func (m *Miner) logClaimedDrops(drops []twitch.ClaimedDrop) {
	for _, drop := range drops {
		reward := drop.RewardName
		if reward == "" {
			reward = "Drop"
		}
		campaign := drop.CampaignName
		if campaign == "" {
			campaign = "Unknown Campaign"
		}
		progress := formatDropProgress(drop.CurrentValue, drop.RequiredValue)
		percent := progressPercent(drop.CurrentValue, drop.RequiredValue)
		m.logger.EmojiEventf(":package:", events.EventDropClaim, "Claim %s (%s) %s (%d%%)", reward, campaign, progress, percent)
	}
}

func (m *Miner) logDropStatuses(statuses []twitch.DropStatus) {
	if m == nil || m.logger == nil || !m.showDropsProgress {
		return
	}
	for _, status := range statuses {
		line := formatDropStatusLine(status)
		if line == "" {
			continue
		}
		m.logger.Eventf(events.EventDropStatus, "%s", line)
	}
}
