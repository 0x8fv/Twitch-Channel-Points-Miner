package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/ansi"
	"TwitchChannelPointsMiner/internal/jsonfile"
	"TwitchChannelPointsMiner/internal/logging"
)

type privacyConfig struct {
	AnonymizeLogs bool `json:"anonymize_logs"`
}

type discordConfig struct {
	WebhookAPI string   `json:"webhook_api"`
	Events     []string `json:"events"`
}

type config struct {
	Username                   string          `json:"username"`
	Password                   string          `json:"password"`
	AutoUpdate                 bool            `json:"auto_update"`
	Debug                      bool            `json:"debug"`
	DebugDeep                  bool            `json:"debug_deep"`
	WatchQueueLogging          bool            `json:"watch_queue_logging"`
	SmartLogging               bool            `json:"smart_logging"`
	DisableSSLCertVerification bool            `json:"disable_ssl_cert_verification"`
	ShowSeconds                bool            `json:"show_seconds"`
	ClaimDropsStartup          bool            `json:"claim_drops_startup"`
	ClaimDrops                 bool            `json:"claim_drops"`
	ClaimMoments               bool            `json:"claim_moments"`
	BettingMakePredictions     bool            `json:"betting(make_predictions)"`
	FollowRaid                 bool            `json:"follow_raid"`
	CommunityGoals             bool            `json:"community_goals"`
	Emojis                     bool            `json:"emojis"`
	SaveLogs                   bool            `json:"save_logs"`
	ShowUsernameInConsole      bool            `json:"show_username_in_console"`
	ShowClaimedBonusMsg        bool            `json:"show_claimed_bonus_msg"`
	ShowGame                   bool            `json:"show_game"`
	WatchStreakWarmStartCache  bool            `json:"watch_streak_warm_start_cache"`
	IRCMode                    string          `json:"chat_presence"`
	DisableAtInNickname        bool            `json:"disable_at_in_nickname"`
	ShowDropsProgress          bool            `json:"show_drops_progress"`
	Streamers                  []string        `json:"streamers"`
	StreamersExclude           []string        `json:"streamers_exclude"`
	GamePriority               []string        `json:"game_priority"`
	GameExclude                []string        `json:"game_exclude"`
	WatchPriority              []string        `json:"watch_priority"`
	Bet                        betConfig       `json:"bet"`
	Timezone                   *string         `json:"timezone"`
	Privacy                    privacyConfig   `json:"privacy"`
	Discord                    discordConfig   `json:"discord"`
	Analytics                  analyticsConfig `json:"analytics"`
	Telegram                   telegramConfig  `json:"telegram"`

	StreamerOverrides map[string]streamerSettingsConfig `json:"streamer_overrides"`
}

func defaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"username":                      "your-twitch-username",
		"password":                      "",
		"auto_update":                   true,
		"debug":                         false,
		"debug_deep":                    false,
		"watch_queue_logging":           false,
		"smart_logging":                 true,
		"disable_ssl_cert_verification": false,
		"show_seconds":                  false,
		"claim_drops_startup":           true,
		"claim_drops":                   true,
		"claim_moments":                 true,
		"betting(make_predictions)":     true,
		"follow_raid":                   true,
		"community_goals":               false,
		"emojis":                        true,
		"save_logs":                     false,
		"show_username_in_console":      false,
		"show_claimed_bonus_msg":        true,
		"show_game":                     true,
		"watch_streak_warm_start_cache": true,
		"chat_presence":                 "ONLINE",
		"disable_at_in_nickname":        false,
		"show_drops_progress":           false,
		"timezone":                      nil,
		"privacy": map[string]interface{}{
			"anonymize_logs": false,
		},
		"discord": map[string]interface{}{
			"webhook_api": "",
			"events":      []interface{}{},
		},
		"analytics":         defaultAnalyticsConfig(),
		"telegram":          defaultTelegramConfig(),
		"streamers":         []interface{}{},
		"streamers_exclude": []interface{}{},
		"game_priority":     []interface{}{},
		"game_exclude":      []interface{}{},
		"watch_priority": []interface{}{
			"STREAK",
			"DROPS",
			"ORDER",
		},
		"streamer_overrides": map[string]interface{}{},
		"bet": map[string]interface{}{
			"strategy":              "SMART",
			"percentage":            5,
			"percentage_gap":        20,
			"max_points":            50000,
			"stealth_mode":          false,
			"deduct_stake_on_place": true,
			"delay_mode":            "FROM_END",
			"delay":                 6,
			"minimum_points":        0,
			"filter_condition": map[string]interface{}{
				"by":    nil,
				"where": nil,
				"value": nil,
			},
		},
	}
}

func loadOrCreateConfig(path string) (config, error) {
	cfgMap := map[string]interface{}{}
	fileData, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(fileData, &cfgMap); err != nil {
			return config{}, fmt.Errorf("invalid config: %w", err)
		}
	}

	changed := false
	for key, value := range defaultConfig() {
		if _, ok := cfgMap[key]; !ok {
			cfgMap[key] = value
			changed = true
		}
	}

	privacyRaw, ok := cfgMap["privacy"].(map[string]interface{})
	if !ok {
		privacyRaw = defaultConfig()["privacy"].(map[string]interface{})
		cfgMap["privacy"] = privacyRaw
		changed = true
	} else {
		defaultPrivacy := defaultConfig()["privacy"].(map[string]interface{})
		for k, v := range defaultPrivacy {
			if _, ok := privacyRaw[k]; !ok {
				privacyRaw[k] = v
				changed = true
			}
		}
	}

	discordRaw, ok := cfgMap["discord"].(map[string]interface{})
	if !ok {
		discordRaw = defaultConfig()["discord"].(map[string]interface{})
		cfgMap["discord"] = discordRaw
		changed = true
	} else {
		defaultDiscord := defaultConfig()["discord"].(map[string]interface{})
		for k, v := range defaultDiscord {
			if _, ok := discordRaw[k]; !ok {
				discordRaw[k] = v
				changed = true
			}
		}
	}

	betRaw, ok := cfgMap["bet"].(map[string]interface{})
	if !ok {
		betRaw = defaultConfig()["bet"].(map[string]interface{})
		cfgMap["bet"] = betRaw
		changed = true
	} else {
		defaultBet := defaultConfig()["bet"].(map[string]interface{})
		for k, v := range defaultBet {
			if _, ok := betRaw[k]; !ok {
				betRaw[k] = v
				changed = true
			}
		}
		// ? Ensure nested filter_condition keys are present.
		if fcRaw, ok := betRaw["filter_condition"].(map[string]interface{}); ok {
			for k, v := range defaultBet["filter_condition"].(map[string]interface{}) {
				if _, ok := fcRaw[k]; !ok {
					fcRaw[k] = v
					changed = true
				}
			}
		} else {
			betRaw["filter_condition"] = defaultBet["filter_condition"]
			changed = true
		}
	}

	if err != nil || changed {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return config{}, err
		}
		if err := jsonfile.SaveJSON(path, cfgMap); err != nil {
			return config{}, err
		}
	}

	normalized, err := json.Marshal(cfgMap)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(normalized, &cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func applyTimezoneOverride(raw *string, logger *logging.Logger) {
	if raw == nil {
		return
	}
	zone := strings.TrimSpace(*raw)
	if zone == "" || strings.EqualFold(zone, "auto") {
		return
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		logger.Errorf("%sTimezone override ignored; falling back to system time: %v%s", ansi.ColorRed, err, ansi.ColorReset)
	}
	time.Local = loc
}
