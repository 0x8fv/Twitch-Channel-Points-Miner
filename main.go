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

type privacyConfig struct {
	AnonymizeLogs bool `json:"anonymize_logs"`
}

type discordConfig struct {
	WebhookAPI string   `json:"webhook_api"`
	Events     []string `json:"events"`
}

type config struct {
	Username                   string `json:"username"`
	Password                   string `json:"password"`
	AutoUpdate                 bool   `json:"auto_update"`
	Debug                      bool   `json:"debug"`
	DebugDeep                  bool   `json:"debug_deep"`
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
	IRCMode                    string `json:"chat_presence"`
	// ShowDropsIndicator         bool      `json:"show_drops_indicator"`
	Streamers        []string      `json:"streamers"`
	StreamersExclude []string      `json:"streamers_exclude"`
	GamePriority     []string      `json:"game_priority"`
	GameExclude      []string      `json:"game_exclude"`
	WatchPriority    []string      `json:"watch_priority"`
	Bet              betConfig     `json:"bet"`
	Timezone         *string       `json:"timezone"`
	Privacy          privacyConfig `json:"privacy"`
	Discord          discordConfig `json:"discord"`

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
	if override.DeductStakeOnPlace != nil {
		out.DeductStakeOnPlace = override.DeductStakeOnPlace
	}
	if override.FilterCondition != nil {
		out.FilterCondition = mergeFilterCondition(out.FilterCondition, override.FilterCondition)
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
	if override.IRCMode != nil {
		out.IRCMode = parseChatPresence(*override.IRCMode, out.IRCMode)
	}
	out.Default()
	return out
}

func parseChatPresence(mode string, fallback entities.IRCMode) entities.IRCMode {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case string(entities.IRCModeAlways):
		return entities.IRCModeAlways
	case string(entities.IRCModeNever):
		return entities.IRCModeNever
	case string(entities.IRCModeOffline):
		return entities.IRCModeOffline
	case string(entities.IRCModeOnline):
		return entities.IRCModeOnline
	default:
		return fallback
	}
}

func mergeFilterCondition(base *entities.FilterCondition, override *filterConditionConfig) *entities.FilterCondition {
	if override == nil {
		return base
	}
	var out entities.FilterCondition
	if base != nil {
		out = *base
	}
	if override.By != "" {
		out.By = entities.OutcomeKey(strings.ToUpper(strings.TrimSpace(override.By)))
	}
	if override.Where != "" {
		out.Where = entities.Condition(strings.ToUpper(strings.TrimSpace(override.Where)))
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
		"debug_deep":                    false,
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
		"chat_presence":                 "ONLINE",
		"timezone":                      nil,
		"privacy": map[string]interface{}{
			"anonymize_logs": false,
		},
		"discord": map[string]interface{}{
			"webhook_api": "",
			"events":      []interface{}{},
		},
		// "show_drops_indicator":          true,
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
			"strategy":              nil,
			"percentage":            nil,
			"percentage_gap":        nil,
			"max_points":            nil,
			"stealth_mode":          nil,
			"deduct_stake_on_place": true,
			"delay_mode":            nil,
			"delay":                 nil,
			"minimum_points":        nil,
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

func buildBaseStreamerSettings(cfg config) entities.StreamerSettings {
	betSettings := entities.BetSettings{
		Strategy:           entities.Strategy(cfg.Bet.Strategy),
		Percentage:         cfg.Bet.Percentage,
		PercentageGap:      cfg.Bet.PercentageGap,
		MaxPoints:          cfg.Bet.MaxPoints,
		StealthMode:        cfg.Bet.StealthMode,
		DeductStakeOnPlace: cfg.Bet.DeductStakeOnPlace,
		DelayMode:          entities.DelayMode(cfg.Bet.DelayMode),
		Delay:              cfg.Bet.Delay,
		MinimumPoints:      cfg.Bet.MinimumPoints,
		FilterCondition:    mergeFilterCondition(nil, cfg.Bet.FilterCondition),
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
		IRCMode:         parseChatPresence(cfg.IRCMode, entities.IRCModeOnline),
	}
	streamerSettings.Default()
	return streamerSettings
}

func buildOverrideSettings(base entities.StreamerSettings, overrides map[string]streamerSettingsConfig) map[string]entities.StreamerSettings {
	overrideSettings := make(map[string]entities.StreamerSettings, len(overrides))
	for name, override := range overrides {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		overrideSettings[key] = mergeStreamerSettings(base, override)
	}
	return overrideSettings
}

func main() {
	setConsoleTitle("Klaro's Twitch Miner")
	clearConsole()
	cfg, err := loadOrCreateConfig("config.json")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.AutoUpdate {
		updated, err := miner.RunAutoUpdate()
		if err != nil {
			log.Printf("auto-update failed: %v", err)
		}
		if updated {
			log.Printf("auto-update installed a newer version; restarting...")
			return
		}
	}

	// ? Apply optional defaults/overrides (per-streamer)
	baseStreamerSettings := buildBaseStreamerSettings(cfg)
	overrideSettings := buildOverrideSettings(baseStreamerSettings, cfg.StreamerOverrides)

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
		DebugDeep:        cfg.DebugDeep,
		AnonymizeLogs:    cfg.Privacy.AnonymizeLogs,
		Discord: miner.DiscordSettings{
			WebhookAPI: cfg.Discord.WebhookAPI,
			Events:     cfg.Discord.Events,
		},
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
		cfg.StreamersExclude,
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
