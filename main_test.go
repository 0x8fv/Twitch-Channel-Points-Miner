package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	miner "TwitchChannelPointsMiner/TwitchChannelPointsMiner"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestDefaultConfigIncludesExpectedKeys(t *testing.T) {
	cfg := defaultConfig()
	required := []string{
		"username",
		"password",
		"streamers",
		"game_priority",
		"chat_presence",
		"bet",
		"watch_priority",
	}
	for _, key := range required {
		if _, ok := cfg[key]; !ok {
			t.Fatalf("missing key %q in default config", key)
		}
	}
}

func TestLoadOrCreateConfigCreatesFileAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")

	cfg, err := loadOrCreateConfig(path)
	if err != nil {
		t.Fatalf("load/create config error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file should be created: %v", err)
	}
	if cfg.WatchPriority == nil || len(cfg.WatchPriority) == 0 {
		t.Fatalf("watch priority should have defaults: %#v", cfg)
	}
}

func TestApplyTimezoneOverride(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	zone := "UTC"
	logger := miner.NewLogger(miner.LoggerSettings{}, "")
	applyTimezoneOverride(&zone, logger)
	if time.Local.String() != "UTC" {
		t.Fatalf("expected time.Local set to UTC, got %s", time.Local.String())
	}

	bad := "Bad/Zone"
	applyTimezoneOverride(&bad, logger)
	if time.Local.String() != "UTC" {
		t.Fatalf("invalid timezone should not change location, got %s", time.Local.String())
	}
}

func TestBuildBaseStreamerSettingsAppliesGlobalFilterCondition(t *testing.T) {
	cfg := config{
		BettingMakePredictions: true,
		FollowRaid:             true,
		ClaimDrops:             true,
		CommunityGoals:         false,
		IRCMode:                "ONLINE",
		Bet: betConfig{
			FilterCondition: &filterConditionConfig{
				By:    "TOTAL_USERS",
				Where: "GTE",
				Value: func() *float64 { v := 500000.0; return &v }(),
			},
		},
	}

	base := buildBaseStreamerSettings(cfg)
	if base.Bet.FilterCondition == nil {
		t.Fatalf("expected global filter_condition applied to base streamer settings")
	}
	if base.Bet.FilterCondition.By != "TOTAL_USERS" {
		t.Fatalf("expected By TOTAL_USERS, got %s", base.Bet.FilterCondition.By)
	}
	if base.Bet.FilterCondition.Where != "GTE" {
		t.Fatalf("expected Where GTE, got %s", base.Bet.FilterCondition.Where)
	}
	if base.Bet.FilterCondition.Value == nil || *base.Bet.FilterCondition.Value != 500000.0 {
		t.Fatalf("expected Value 500000, got %#v", base.Bet.FilterCondition.Value)
	}
}

func TestBuildOverrideSettingsMergesFilterCondition(t *testing.T) {
	base := entities.StreamerSettings{
		MakePredictions: true,
		Bet: entities.BetSettings{
			Strategy: entities.StrategySmart,
		},
	}
	base.Default()

	overrides := map[string]streamerSettingsConfig{
		"SomeStreamer": {
			Bet: betConfig{
				FilterCondition: &filterConditionConfig{
					By:    "TOTAL_POINTS",
					Where: "GT",
					Value: func() *float64 { v := 999999.0; return &v }(),
				},
			},
		},
	}

	merged := buildOverrideSettings(base, overrides)
	override, ok := merged["somestreamer"]
	if !ok {
		t.Fatalf("expected override settings keyed by lowercased streamer name")
	}
	if override.Bet.FilterCondition == nil {
		t.Fatalf("expected override filter_condition merged into settings")
	}
	if override.Bet.FilterCondition.By != "TOTAL_POINTS" || override.Bet.FilterCondition.Where != "GT" {
		t.Fatalf("expected override filter_condition TOTAL_POINTS GT, got %v %v", override.Bet.FilterCondition.By, override.Bet.FilterCondition.Where)
	}
	if override.Bet.FilterCondition.Value == nil || *override.Bet.FilterCondition.Value != 999999.0 {
		t.Fatalf("expected override Value 999999, got %#v", override.Bet.FilterCondition.Value)
	}
}
