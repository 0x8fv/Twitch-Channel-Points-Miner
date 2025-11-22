package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	miner "TwitchChannelPointsMiner/TwitchChannelPointsMiner"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
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

type config struct {
	Username                   string    `json:"username"`
	Password                   string    `json:"password"`
	AutoUpdate                 bool      `json:"auto_update"`
	Debug                      bool      `json:"debug"`
	SmartLogging               bool      `json:"smart_logging"`
	DisableSSLCertVerification bool      `json:"disable_ssl_cert_verification"`
	ShowSeconds                bool      `json:"show_seconds"`
	ClaimDropsStartup          bool      `json:"claim_drops_startup"`
	ClaimDrops                 bool      `json:"claim_drops"`
	BettingMakePredictions     bool      `json:"betting(make_predictions)"`
	FollowRaid                 bool      `json:"follow_raid"`
	CommunityGoals             bool      `json:"community_goals"`
	Emojis                     bool      `json:"emojis"`
	SaveLogs                   bool      `json:"save_logs"`
	ShowUsernameInConsole      bool      `json:"show_username_in_console"`
	ShowClaimedBonusMsg        bool      `json:"show_claimed_bonus_msg"`
	Streamers                  []string  `json:"streamers"`
	WatchPriority              []string  `json:"watch_priority"`
	Bet                        betConfig `json:"bet"`
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
		"streamers":                     []interface{}{},
		"watch_priority": []interface{}{
			"STREAK",
			"DROPS",
			"ORDER",
		},
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

	minr := miner.NewMiner(
		cfg.Username,
		cfg.Password,
		cfg.ClaimDropsStartup,
		cfg.DisableSSLCertVerification,
		loggerSettings,
		streamerSettings,
		cfg.WatchPriority,
	)

	if len(cfg.Streamers) > 0 {
		minr.Mine(cfg.Streamers)
	} else {
		minr.MineFollowers(entities.FollowersOrderDESC)
	}
}
