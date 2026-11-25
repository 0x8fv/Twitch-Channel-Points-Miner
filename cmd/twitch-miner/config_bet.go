package main

import (
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
)

type filterConditionConfig struct {
	By    string   `json:"by"`
	Where string   `json:"where"`
	Value *float64 `json:"value"`
}

type betConfig struct {
	Strategy           string                 `json:"strategy"`
	Percentage         *int                   `json:"percentage"`
	PercentageGap      *int                   `json:"percentage_gap"`
	MaxPoints          *int                   `json:"max_points"`
	StealthMode        *bool                  `json:"stealth_mode"`
	DeductStakeOnPlace *bool                  `json:"deduct_stake_on_place"`
	DelayMode          string                 `json:"delay_mode"`
	Delay              *float64               `json:"delay"`
	MinimumPoints      *int                   `json:"minimum_points"`
	FilterCondition    *filterConditionConfig `json:"filter_condition"`
}

func mergeBetSettings(base domain.BetSettings, override betConfig) domain.BetSettings {
	out := base
	if override.Strategy != "" {
		out.Strategy = domain.Strategy(override.Strategy)
	}
	if override.Percentage != nil {
		out.Percentage = override.Percentage
	}
	if override.PercentageGap != nil {
		out.PercentageGap = override.PercentageGap
	}
	if override.MaxPoints != nil {
		out.MaxPoints = override.MaxPoints
	}
	if override.MinimumPoints != nil {
		out.MinimumPoints = override.MinimumPoints
	}
	if override.StealthMode != nil {
		out.StealthMode = override.StealthMode
	}
	if override.DeductStakeOnPlace != nil {
		out.DeductStakeOnPlace = override.DeductStakeOnPlace
	}
	if override.FilterCondition != nil {
		out.FilterCondition = mergeFilterCondition(out.FilterCondition, override.FilterCondition)
	}
	if override.DelayMode != "" {
		out.DelayMode = domain.DelayMode(override.DelayMode)
	}
	if override.Delay != nil {
		out.Delay = override.Delay
	}
	out.Default()
	return out
}

func mergeFilterCondition(base *domain.FilterCondition, override *filterConditionConfig) *domain.FilterCondition {
	if override == nil {
		return base
	}
	var out domain.FilterCondition
	if base != nil {
		out = *base
	}
	if override.By != "" {
		out.By = domain.OutcomeKey(strings.ToUpper(strings.TrimSpace(override.By)))
	}
	if override.Where != "" {
		out.Where = domain.Condition(strings.ToUpper(strings.TrimSpace(override.Where)))
	}
	if override.Value != nil {
		out.Value = override.Value
	}
	// ? If nothing was set, keep nil to avoid activating an empty filter
	if out.By == "" && out.Where == "" && out.Value == nil {
		return base
	}
	return &out
}
