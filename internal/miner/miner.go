package miner

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/internal/ansi"
	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/privacy"
	"TwitchChannelPointsMiner/internal/twitch"
)

const (
	colorGreen       = ansi.ColorGreen
	colorRed         = ansi.ColorRed
	colorCyan        = ansi.ColorCyan
	colorGameLabel   = ansi.ColorPurple
	colorDropsAccent = ansi.ColorYellow
	colorReset       = ansi.ColorReset
)

func normalizeGameList(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, raw := range values {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}

func normalizeStreamerList(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, raw := range values {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	return normalized
}

type Miner struct {
	Username                   string
	Password                   string
	ClaimDropsStartup          bool
	DisableSSLCertVerification bool
	LoggerSettings             LoggerSettings
	StreamerSettings           domain.StreamerSettings
	StreamerOverrides          map[string]domain.StreamerSettings
	WatchStreakWarmStartCache  bool
	logger                     *Logger
	startedAt                  time.Time
	twitch                     *twitch.Twitch
	streamers                  []*domain.Streamer
	initialPoints              map[string]int
	stop                       chan struct{}
	watchPriorities            []watchPriority
	streamerExclusions         map[string]struct{}
	gamePriority               []string
	gamePriorityIndex          map[string]int
	gameExclusions             map[string]struct{}
	chatWatchers               map[string]*twitch.ChatClient
	chatMu                     sync.Mutex
	disableAtInNickname        bool
	showGameInfo               bool
	showDropsProgress          bool
	logWatchQueue              bool
	anonymizer                 *privacy.Anonymizer
	warmStartCache             *watchStreakWarmStartCache
	warmStartCachePath         string
	activeStreakWatches        map[string]activeStreakWatch
	activeStreakMu             sync.Mutex
}

func NewMiner(username, password string, claimDropsStartup bool, disableCertCheck bool, loggerSettings LoggerSettings, streamerSettings domain.StreamerSettings, streamerOverrides map[string]domain.StreamerSettings, priorityNames []string, streamerExclude []string, gamePriority []string, gameExclude []string, disableAtInNickname bool, showGameInfo bool, logWatchQueue bool, watchStreakWarmStartCache bool, showDropsProgress bool) *Miner {
	streamerSettings.Default()
	priorityList := normalizeGameList(gamePriority)
	excludedGames := make(map[string]struct{})
	for _, name := range normalizeGameList(gameExclude) {
		excludedGames[name] = struct{}{}
	}
	excludedStreamers := make(map[string]struct{})
	for _, name := range normalizeStreamerList(streamerExclude) {
		excludedStreamers[name] = struct{}{}
	}
	priorityIndex := make(map[string]int, len(priorityList))
	for idx, name := range priorityList {
		priorityIndex[name] = idx
	}
	safeUsername := sanitizeFilename(strings.ToLower(strings.TrimSpace(username)))
	warmStartCachePath := ""
	if safeUsername != "" {
		warmStartCachePath = filepath.Join("log", fmt.Sprintf("watch_streak_cache.%s.json", safeUsername))
	}
	return &Miner{
		Username:                   username,
		Password:                   password,
		ClaimDropsStartup:          claimDropsStartup,
		DisableSSLCertVerification: disableCertCheck,
		LoggerSettings:             loggerSettings,
		StreamerSettings:           streamerSettings,
		StreamerOverrides:          streamerOverrides,
		WatchStreakWarmStartCache:  watchStreakWarmStartCache,
		logger:                     NewLogger(loggerSettings, username),
		watchPriorities:            parseWatchPriorities(priorityNames),
		streamerExclusions:         excludedStreamers,
		gamePriority:               priorityList,
		gamePriorityIndex:          priorityIndex,
		gameExclusions:             excludedGames,
		chatWatchers:               make(map[string]*twitch.ChatClient),
		disableAtInNickname:        disableAtInNickname,
		showGameInfo:               showGameInfo,
		showDropsProgress:          showDropsProgress,
		logWatchQueue:              logWatchQueue,
		anonymizer:                 privacy.New(loggerSettings.AnonymizeLogs),
		warmStartCachePath:         warmStartCachePath,
		activeStreakWatches:        make(map[string]activeStreakWatch),
	}
}

// ? Mine runs the miner for an explicit list of streamers.
func (m *Miner) Mine(streamers []string) {
	m.run(streamers, false, domain.FollowersOrderASC)
}

// ? MineFollowers runs the miner using the follower list.
func (m *Miner) MineFollowers(order domain.FollowersOrder) {
	m.run(nil, true, order)
}

func (m *Miner) filterExcludedTargets(targets []string) ([]string, int) {
	if len(targets) == 0 || len(m.streamerExclusions) == 0 {
		return targets, 0
	}
	filtered := make([]string, 0, len(targets))
	excluded := 0
	for _, name := range targets {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := m.streamerExclusions[key]; ok {
			excluded++
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered, excluded
}
