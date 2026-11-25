package main

import (
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
)

type streamerSettingsConfig struct {
	MakePredictions *bool     `json:"make_predictions"`
	FollowRaid      *bool     `json:"follow_raid"`
	ClaimDrops      *bool     `json:"claim_drops"`
	ClaimMoments    *bool     `json:"claim_moments"`
	WatchStreak     *bool     `json:"watch_streak"`
	CommunityGoals  *bool     `json:"community_goals"`
	Bet             betConfig `json:"bet"`
	IRCMode         *string   `json:"chat_presence"`
}

func mergeStreamerSettings(base domain.StreamerSettings, override streamerSettingsConfig) domain.StreamerSettings {
	out := base
	if override.MakePredictions != nil {
		out.MakePredictions = *override.MakePredictions
	}
	if override.FollowRaid != nil {
		out.FollowRaid = *override.FollowRaid
	}
	if override.ClaimDrops != nil {
		out.ClaimDrops = *override.ClaimDrops
	}
	if override.ClaimMoments != nil {
		out.ClaimMoments = *override.ClaimMoments
	}
	if override.WatchStreak != nil {
		out.WatchStreak = *override.WatchStreak
	}
	if override.CommunityGoals != nil {
		out.CommunityGoals = *override.CommunityGoals
	}
	out.Bet = mergeBetSettings(out.Bet, override.Bet)
	if override.IRCMode != nil {
		out.IRCMode = parseChatPresence(*override.IRCMode, out.IRCMode)
	}
	out.Default()
	return out
}

func parseChatPresence(mode string, fallback domain.IRCMode) domain.IRCMode {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case string(domain.IRCModeAlways):
		return domain.IRCModeAlways
	case string(domain.IRCModeNever):
		return domain.IRCModeNever
	case string(domain.IRCModeOffline):
		return domain.IRCModeOffline
	case string(domain.IRCModeOnline):
		return domain.IRCModeOnline
	default:
		return fallback
	}
}

func buildBaseStreamerSettings(cfg config) domain.StreamerSettings {
	betSettings := domain.BetSettings{
		Strategy:           domain.Strategy(cfg.Bet.Strategy),
		Percentage:         cfg.Bet.Percentage,
		PercentageGap:      cfg.Bet.PercentageGap,
		MaxPoints:          cfg.Bet.MaxPoints,
		StealthMode:        cfg.Bet.StealthMode,
		DeductStakeOnPlace: cfg.Bet.DeductStakeOnPlace,
		DelayMode:          domain.DelayMode(cfg.Bet.DelayMode),
		Delay:              cfg.Bet.Delay,
		MinimumPoints:      cfg.Bet.MinimumPoints,
		FilterCondition:    mergeFilterCondition(nil, cfg.Bet.FilterCondition),
	}
	betSettings.Default()

	streamerSettings := domain.StreamerSettings{
		MakePredictions: cfg.BettingMakePredictions,
		FollowRaid:      cfg.FollowRaid,
		ClaimDrops:      cfg.ClaimDrops,
		ClaimMoments:    cfg.ClaimMoments,
		WatchStreak:     true,
		CommunityGoals:  cfg.CommunityGoals,
		Bet:             betSettings,
		IRCMode:         parseChatPresence(cfg.IRCMode, domain.IRCModeOnline),
	}
	streamerSettings.Default()
	return streamerSettings
}

func buildOverrideSettings(base domain.StreamerSettings, overrides map[string]streamerSettingsConfig) map[string]domain.StreamerSettings {
	overrideSettings := make(map[string]domain.StreamerSettings, len(overrides))
	for name, override := range overrides {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		overrideSettings[key] = mergeStreamerSettings(base, override)
	}
	return overrideSettings
}
