package drops

import (
	"fmt"
	"strings"

	"TwitchChannelPointsMiner/internal/twitch"
)

func formatDropProgress(current, required int) string {
	if required > 0 {
		return fmt.Sprintf("%d/%d", current, required)
	}
	return fmt.Sprintf("%d", current)
}

func formatDropProgressBar(current, required int) string {
	const width = 20
	percent := progressPercent(current, required)
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := (percent * width) / 100
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func formatDropStatusLine(status twitch.DropStatus) string {
	reward := strings.TrimSpace(status.RewardName)
	if reward == "" {
		reward = "Drop"
	}
	campaign := strings.TrimSpace(status.CampaignName)
	game := strings.TrimSpace(status.GameName)
	if game == "" {
		game = campaign
	}
	if campaign == "" {
		campaign = "Unknown Campaign"
	}
	title := formatDropStatusTitle(game, campaign, reward)
	channel := strings.TrimSpace(status.ChannelName)
	if channel == "" {
		channel = "unknown_streamer"
	}

	current := status.CurrentValue
	required := status.RequiredValue
	if current < 0 {
		current = 0
	}
	remaining := 0
	if required > current {
		remaining = required - current
	}
	if required <= 0 {
		return fmt.Sprintf(
			"DROP_PROGRESS %s %d%% | %s / %s / %dm watched / %s",
			formatDropProgressBar(current, required),
			progressPercent(current, required),
			title,
			channel,
			current,
			dropStatusLabel(status),
		)
	}
	return fmt.Sprintf(
		"DROP_PROGRESS %s %d%% | %s / %s / %dm of %dm / remaining %dm / %s",
		formatDropProgressBar(current, required),
		progressPercent(current, required),
		title,
		channel,
		current,
		required,
		remaining,
		dropStatusLabel(status),
	)
}

func formatDropStatusTitle(game, campaign, reward string) string {
	parts := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range parts {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		parts = append(parts, value)
	}
	add(game)
	add(campaign)
	add(reward)
	return strings.Join(parts, " / ")
}

func dropStatusLabel(status twitch.DropStatus) string {
	if status.Claimed {
		return "CLAIMED"
	}
	if status.Claimable || (status.RequiredValue > 0 && status.CurrentValue >= status.RequiredValue) {
		return "CLAIMABLE"
	}
	return "IN_PROGRESS"
}

func progressPercent(current, required int) int {
	if required <= 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	percent := (current * 100) / required
	if percent < 0 {
		return 0
	}
	return percent
}
