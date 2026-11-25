package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/logging"
	"TwitchChannelPointsMiner/internal/miner"
	"TwitchChannelPointsMiner/internal/notify"
	"TwitchChannelPointsMiner/internal/update"
)

func main() {
	configFlag := flag.String("config", "", "Path to config.json (default: ./config.json or next to the executable)")
	dataDirFlag := flag.String("data-dir", "", "Directory for config/cookies/log (default: current directory if config.json exists; otherwise the executable directory)")
	flag.Parse()

	hasOverride := *configFlag != "" || *dataDirFlag != "" || strings.TrimSpace(os.Getenv("TCPM_CONFIG")) != "" || strings.TrimSpace(os.Getenv("TCPM_DATA_DIR")) != ""
	paths, err := resolveAppPaths(*configFlag, *dataDirFlag)
	if err != nil {
		log.Fatalf("failed to resolve config paths: %v", err)
	}
	if paths.WorkDir != "" {
		_ = os.MkdirAll(paths.WorkDir, 0o755)
		if err := os.Chdir(paths.WorkDir); err != nil {
			log.Printf("warning: failed to change working directory to %q: %v", paths.WorkDir, err)
		}
	}

	setConsoleTitle("Klaro's Twitch Miner")
	clearConsole()
	cfg, err := loadOrCreateConfig(paths.ConfigPath)
	if err != nil && !hasOverride && shouldFallbackToUserConfig(err) {
		if base, derr := os.UserConfigDir(); derr == nil && base != "" {
			fallbackDir := filepath.Join(base, "TwitchChannelPointsMiner")
			_ = os.MkdirAll(fallbackDir, 0o755)
			if chErr := os.Chdir(fallbackDir); chErr == nil {
				fallbackCfg := filepath.Join(fallbackDir, "config.json")
				if cfg2, err2 := loadOrCreateConfig(fallbackCfg); err2 == nil {
					cfg = cfg2
					err = nil
					log.Printf("using config directory %q", fallbackDir)
				}
			}
		}
	}
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.AutoUpdate && !autoUpdateDisabledByEnv() {
		updated, err := update.RunAutoUpdate()
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

	loggerSettings := logging.LoggerSettings{
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
		Discord: notify.DiscordSettings{
			WebhookAPI: cfg.Discord.WebhookAPI,
			Events:     cfg.Discord.Events,
		},
	}

	logger := logging.NewLogger(loggerSettings, cfg.Username)
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
		cfg.DisableAtInNickname,
		cfg.ShowGame,
		cfg.WatchQueueLogging,
		cfg.WatchStreakWarmStartCache,
		cfg.ShowDropsProgress,
	)
	startAnalytics(cfg.Analytics, logger)
	startTelegram(cfg.Telegram)

	if len(cfg.Streamers) > 0 {
		minr.Mine(cfg.Streamers)
	} else {
		minr.MineFollowers(domain.FollowersOrderDESC)
	}
}
