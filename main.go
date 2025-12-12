package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	miner "TwitchChannelPointsMiner/TwitchChannelPointsMiner"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

type betConfig struct {
	Strategy      string   `json:"strategy"`
	Percentage    *int     `json:"percentage"`
	PercentageGap *int     `json:"percentage_gap"`
	MaxPoints     *int     `json:"max_points"`
	StealthMode   *bool    `json:"stealth_mode"`
	DelayMode     string   `json:"delay_mode"`
	Delay         *float64 `json:"delay"`
	MinimumPoints *int     `json:"minimum_points"`
}

type streamerSettingsConfig struct {
	MakePredictions *bool     `json:"make_predictions"`
	FollowRaid      *bool     `json:"follow_raid"`
	ClaimDrops      *bool     `json:"claim_drops"`
	ClaimMoments    *bool     `json:"claim_moments"`
	WatchStreak     *bool     `json:"watch_streak"`
	CommunityGoals  *bool     `json:"community_goals"`
	Bet             betConfig `json:"bet"`
}

type config struct {
	Username                   string `json:"username"`
	Password                   string `json:"password"`
	AutoUpdate                 bool   `json:"auto_update"`
	Debug                      bool   `json:"debug"`
	WatchQueueLogging          bool   `json:"watch_queue_logging"`
	SmartLogging               bool   `json:"smart_logging"`
	DisableSSLCertVerification bool   `json:"disable_ssl_cert_verification"`
	ShowSeconds                bool   `json:"show_seconds"`
	ClaimDropsStartup          bool   `json:"claim_drops_startup"`
	ClaimDrops                 bool   `json:"claim_drops"`
	BettingMakePredictions     bool   `json:"betting(make_predictions)"`
	FollowRaid                 bool   `json:"follow_raid"`
	CommunityGoals             bool   `json:"community_goals"`
	Emojis                     bool   `json:"emojis"`
	SaveLogs                   bool   `json:"save_logs"`
	ShowUsernameInConsole      bool   `json:"show_username_in_console"`
	ShowClaimedBonusMsg        bool   `json:"show_claimed_bonus_msg"`
	ShowGame                   bool   `json:"show_game"`
	// ShowDropsIndicator         bool      `json:"show_drops_indicator"`
	Streamers     []string  `json:"streamers"`
	GamePriority  []string  `json:"game_priority"`
	GameExclude   []string  `json:"game_exclude"`
	WatchPriority []string  `json:"watch_priority"`
	Bet           betConfig `json:"bet"`
	Timezone      *string   `json:"timezone"`

	StreamerOverrides map[string]streamerSettingsConfig `json:"streamer_overrides"`
}

func mergeBetSettings(base entities.BetSettings, override betConfig) entities.BetSettings {
	out := base
	if override.Strategy != "" {
		out.Strategy = entities.Strategy(override.Strategy)
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
	if override.DelayMode != "" {
		out.DelayMode = entities.DelayMode(override.DelayMode)
	}
	if override.Delay != nil {
		out.Delay = override.Delay
	}
	out.Default()
	return out
}

func mergeStreamerSettings(base entities.StreamerSettings, override streamerSettingsConfig) entities.StreamerSettings {
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
	out.Default()
	return out
}

func clearConsole() {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", "cls")
	} else {
		c = exec.Command("clear")
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	_ = c.Run()
}

func setConsoleTitle(title string) {
	if runtime.GOOS != "windows" {
		return
	}
	cmd := exec.Command("cmd", "/c", fmt.Sprintf("title %s", title))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func defaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"username":                      "your-twitch-username",
		"password":                      "your-twitch-password (Optional)",
		"auto_update":                   true,
		"debug":                         false,
		"watch_queue_logging":           false,
		"smart_logging":                 true,
		"disable_ssl_cert_verification": false,
		"show_seconds":                  false,
		"claim_drops_startup":           true,
		"claim_drops":                   true,
		"betting(make_predictions)":     true,
		"follow_raid":                   true,
		"community_goals":               false,
		"emojis":                        true,
		"save_logs":                     false,
		"show_username_in_console":      false,
		"show_claimed_bonus_msg":        true,
		"show_game":                     true,
		"timezone":                      nil,
		// "show_drops_indicator":          true,
		"streamers":     []interface{}{},
		"game_priority": []interface{}{},
		"game_exclude":  []interface{}{},
		"watch_priority": []interface{}{
			"STREAK",
			"DROPS",
			"ORDER",
		},
		"streamer_overrides": map[string]interface{}{},
		"bet": map[string]interface{}{
			"strategy":       nil,
			"percentage":     nil,
			"percentage_gap": nil,
			"max_points":     nil,
			"stealth_mode":   nil,
			"delay_mode":     nil,
			"delay":          nil,
			"minimum_points": nil,
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

	betRaw, ok := cfgMap["bet"].(map[string]interface{})
	if !ok {
		betRaw = defaultConfig()["bet"].(map[string]interface{})
		cfgMap["bet"] = betRaw
		changed = true
	} else {
		for k, v := range defaultConfig()["bet"].(map[string]interface{}) {
			if _, ok := betRaw[k]; !ok {
				betRaw[k] = v
				changed = true
			}
		}
	}

	if err != nil || changed {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return config{}, err
		}
		if err := utils.SaveJSON(path, cfgMap); err != nil {
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

func applyTimezoneOverride(raw *string, logger *miner.Logger) {
	if raw == nil {
		return
	}
	zone := strings.TrimSpace(*raw)
	if zone == "" || strings.EqualFold(zone, "auto") {
		return
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		logger.Errorf("%sTimezone override ignored; falling back to system time: %v%s", constants.ColorRed, err, constants.ColorReset)
	}
	time.Local = loc
}

func main() {
	setConsoleTitle("Klaro's Twitch Miner")
	clearConsole()
	cfg, err := loadOrCreateConfig("config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.AutoUpdate {
		updated, err := miner.RunAutoUpdate(cfg.DisableSSLCertVerification)
		if err != nil {
			log.Printf("auto-update failed: %v", err)
		}
		if updated {
			log.Printf("auto-update installed a newer version; restarting...")
			return
		}
	}

	betSettings := entities.BetSettings{
		Strategy:      entities.Strategy(cfg.Bet.Strategy),
		Percentage:    cfg.Bet.Percentage,
		PercentageGap: cfg.Bet.PercentageGap,
		MaxPoints:     cfg.Bet.MaxPoints,
		StealthMode:   cfg.Bet.StealthMode,
		DelayMode:     entities.DelayMode(cfg.Bet.DelayMode),
		Delay:         cfg.Bet.Delay,
		MinimumPoints: cfg.Bet.MinimumPoints,
	}
	betSettings.Default()

	streamerSettings := entities.StreamerSettings{
		MakePredictions: cfg.BettingMakePredictions,
		FollowRaid:      cfg.FollowRaid,
		ClaimDrops:      cfg.ClaimDrops,
		ClaimMoments:    true,
		WatchStreak:     true,
		CommunityGoals:  cfg.CommunityGoals,
		Bet:             betSettings,
	}
	streamerSettings.Default()

	// ? Apply optional defaults/overrides (per-streamer)
	baseStreamerSettings := streamerSettings

	overrideSettings := make(map[string]entities.StreamerSettings, len(cfg.StreamerOverrides))
	for name, override := range cfg.StreamerOverrides {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		overrideSettings[key] = mergeStreamerSettings(baseStreamerSettings, override)
	}

	loggerSettings := miner.LoggerSettings{
		Save:             cfg.SaveLogs,
		ConsoleLevel:     0,
		FileLevel:        0,
		Emoji:            cfg.Emojis,
		Smart:            cfg.SmartLogging,
		ShowSeconds:      cfg.ShowSeconds,
		ConsoleUsername:  cfg.ShowUsernameInConsole,
		ShowClaimedBonus: cfg.ShowClaimedBonusMsg,
		Less:             false,
		Debug:            cfg.Debug,
	}

	logger := miner.NewLogger(loggerSettings, cfg.Username)
	applyTimezoneOverride(cfg.Timezone, logger)

	minr := miner.NewMiner(
		cfg.Username,
		cfg.Password,
		cfg.ClaimDropsStartup,
		cfg.DisableSSLCertVerification,
		loggerSettings,
		baseStreamerSettings,
		overrideSettings,
		cfg.WatchPriority,
		cfg.GamePriority,
		cfg.GameExclude,
		cfg.ShowGame,
		cfg.WatchQueueLogging,
		// cfg.ShowDropsIndicator,
	)

	if len(cfg.Streamers) > 0 {
		minr.Mine(cfg.Streamers)
	} else {
		minr.MineFollowers(entities.FollowersOrderDESC)
	}
}
