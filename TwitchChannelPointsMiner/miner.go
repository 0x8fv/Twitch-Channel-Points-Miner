package twitchchannelpointsminer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	classpkg "TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/privacy"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

const (
	colorGreen       = constants.ColorGreen
	colorRed         = constants.ColorRed
	colorCyan        = constants.ColorCyan
	colorGameLabel   = constants.ColorPurple
	colorDropsAccent = constants.ColorYellow
	colorReset       = constants.ColorReset
)

const (
	streakPriorityMinutesBase     = 7.0
	streakPriorityMinutesExtended = 20.0
	resolvedStreakCarryoverWindow = 30 * time.Minute
	falseOfflineStreamStartGrace  = 2 * time.Minute
)

const (
	minuteWatchFailureBackoffThreshold = 3
	minuteWatchFailureBackoffBase      = 5 * time.Minute
	minuteWatchFailureBackoffMax       = 30 * time.Minute
)

type activeStreakWatch struct {
	BroadcastID string
	CreatedAt   time.Time
}

type watchPriority int

const (
	watchPriorityOrder watchPriority = iota
	watchPriorityStreak
	watchPriorityDrops
	watchPrioritySubscribed
	watchPriorityPointsAscending
	watchPriorityPointsDescending
)

const maxConcurrentWatchers = 2

func defaultWatchPriorities() []watchPriority {
	return []watchPriority{
		watchPriorityStreak,
		watchPriorityDrops,
		watchPriorityOrder,
	}
}

func parseWatchPriorities(priorityNames []string) []watchPriority {
	if len(priorityNames) == 0 {
		return defaultWatchPriorities()
	}
	seen := make(map[watchPriority]struct{})
	parsed := make([]watchPriority, 0, len(priorityNames))
	add := func(p watchPriority) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		parsed = append(parsed, p)
	}
	for _, raw := range priorityNames {
		name := strings.ToUpper(strings.TrimSpace(raw))
		switch name {
		case "ORDER":
			add(watchPriorityOrder)
		case "STREAK":
			add(watchPriorityStreak)
		case "DROPS":
			add(watchPriorityDrops)
		case "SUBSCRIBED", "SUBS", "MULTIPLIER":
			add(watchPrioritySubscribed)
		case "POINTS_ASC", "POINTS_ASCENDING":
			add(watchPriorityPointsAscending)
		case "POINTS_DESC", "POINTS_DESCENDING":
			add(watchPriorityPointsDescending)
		}
	}
	if len(parsed) == 0 {
		return defaultWatchPriorities()
	}
	return parsed
}

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
	StreamerSettings           entities.StreamerSettings
	StreamerOverrides          map[string]entities.StreamerSettings
	WatchStreakWarmStartCache  bool
	logger                     *Logger
	startedAt                  time.Time
	twitch                     *classpkg.Twitch
	streamers                  []*entities.Streamer
	initialPoints              map[string]int
	stop                       chan struct{}
	watchPriorities            []watchPriority
	streamerExclusions         map[string]struct{}
	gamePriority               []string
	gamePriorityIndex          map[string]int
	gameExclusions             map[string]struct{}
	chatWatchers               map[string]*classpkg.ChatClient
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

func NewMiner(username, password string, claimDropsStartup bool, disableCertCheck bool, loggerSettings LoggerSettings, streamerSettings entities.StreamerSettings, streamerOverrides map[string]entities.StreamerSettings, priorityNames []string, streamerExclude []string, gamePriority []string, gameExclude []string, disableAtInNickname bool, showGameInfo bool, logWatchQueue bool, watchStreakWarmStartCache bool, showDropsProgress bool) *Miner {
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
		chatWatchers:               make(map[string]*classpkg.ChatClient),
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
	m.run(streamers, false, entities.FollowersOrderASC)
}

// ? MineFollowers runs the miner using the follower list.
func (m *Miner) MineFollowers(order entities.FollowersOrder) {
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

func (m *Miner) shouldClaimDrops(streamers []*entities.Streamer) bool {
	if m.StreamerSettings.ClaimDrops {
		return true
	}
	for _, streamer := range streamers {
		if streamer != nil && streamer.Settings.ClaimDrops {
			return true
		}
	}
	return false
}

func (m *Miner) loadWarmStartCache() {
	if m == nil || !m.WatchStreakWarmStartCache {
		return
	}
	m.warmStartCache = loadWatchStreakWarmStartCache(m.warmStartCachePath, m.Username)
}

func (m *Miner) saveWarmStartCacheSnapshot(streamers []*entities.Streamer) {
	if m == nil || m.warmStartCache == nil {
		return
	}
	now := time.Now()
	for _, streamer := range streamers {
		m.warmStartCache.updateFromStreamer(streamer, now)
	}
	if err := m.warmStartCache.saveIfDirty(); err != nil && m.logger != nil {
		m.logger.Debugf("warm-start cache save failed: %v", err)
	}
}

func (m *Miner) syncWarmStartCacheFromStreamer(streamer *entities.Streamer) {
	if m == nil || m.warmStartCache == nil || streamer == nil {
		return
	}
	m.warmStartCache.updateFromStreamer(streamer, time.Now())
	if err := m.warmStartCache.saveIfDirty(); err != nil && m.logger != nil {
		m.logger.Debugf("warm-start cache save failed for %s: %v", m.rawStreamerName(streamer.Username), err)
	}
}

func (m *Miner) applyWarmStartCache(streamer *entities.Streamer) bool {
	if m == nil || m.warmStartCache == nil || streamer == nil || streamer.Stream == nil || !streamer.IsOnline {
		return false
	}
	entry, ok := m.warmStartCache.resolvedEntryForStreamer(streamer, time.Now())
	if !ok {
		return false
	}
	streamer.Stream.WatchStreakMissing = entry.WatchStreakMissing
	return true
}

func streamerInitialPointsKey(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if id := strings.TrimSpace(streamer.ChannelID); id != "" {
		return id
	}
	return strings.ToLower(strings.TrimSpace(streamer.Username))
}

func (m *Miner) applyStreamerLoginChange(streamer *entities.Streamer, login string) bool {
	if streamer == nil {
		return false
	}
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || strings.EqualFold(streamer.Username, login) {
		return false
	}

	oldLogin := strings.ToLower(strings.TrimSpace(streamer.Username))
	if m != nil {
		m.stopChatWatcher(streamer)
	}

	streamer.Username = login
	streamer.StreamerURL = fmt.Sprintf("%s/%s", constants.URL, login)
	if streamer.Stream != nil {
		streamer.Stream.SpadeURL = ""
	}
	if m != nil && m.logger != nil {
		if m.anonymizeLogs() {
			m.logger.Printf("identity refresh %s", m.styledStreamerName(streamer))
		} else {
			m.logger.Printf("identity refresh %s -> %s for channel %s", oldLogin, streamer.Username, streamer.ChannelID)
		}
	}

	if m != nil && m.initialPoints != nil && oldLogin != "" {
		stableKey := streamerInitialPointsKey(streamer)
		if stableKey != "" {
			if initial, ok := m.initialPoints[oldLogin]; ok {
				m.initialPoints[stableKey] = initial
			}
		}
	}
	if m != nil {
		m.updateChatPresence(streamer, streamer.IsOnline)
		m.syncWarmStartCacheFromStreamer(streamer)
	}
	return true
}

func shouldRepairStreamerIdentity(err error) bool {
	return errors.Is(err, classpkg.ErrChannelNotFound)
}

func (m *Miner) repairStreamerIdentity(streamer *entities.Streamer, err error) bool {
	if m == nil || m.twitch == nil || streamer == nil || !shouldRepairStreamerIdentity(err) {
		return false
	}
	user, lookupErr := m.twitch.GetUserByID(streamer.ChannelID)
	if lookupErr != nil {
		if m.logger != nil {
			m.logger.Printf("identity refresh %s: %v", m.styledStreamerName(streamer), lookupErr)
		}
		return false
	}
	if user == nil || user.Login == "" {
		return false
	}
	return m.applyStreamerLoginChange(streamer, user.Login)
}

func (m *Miner) loadChannelPointsContextWithIdentityRepair(streamer *entities.Streamer) (int, error) {
	points, err := m.twitch.LoadChannelPointsContext(streamer)
	if err == nil {
		return points, nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return points, err
	}
	return m.twitch.LoadChannelPointsContext(streamer)
}

func (m *Miner) checkStreamerOnlineWithIdentityRepair(streamer *entities.Streamer) (bool, error) {
	online, err := m.twitch.CheckStreamerOnline(streamer)
	if err == nil {
		return online, nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return online, err
	}
	return m.twitch.CheckStreamerOnline(streamer)
}

func (m *Miner) updateStreamWithIdentityRepair(streamer *entities.Streamer) error {
	err := m.twitch.UpdateStream(streamer)
	if err == nil {
		return nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return err
	}
	return m.twitch.UpdateStream(streamer)
}

func (m *Miner) sendMinuteWatchedWithIdentityRepair(streamer *entities.Streamer) error {
	err := m.twitch.SendMinuteWatched(streamer)
	if err == nil {
		return nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return err
	}
	return m.twitch.SendMinuteWatched(streamer)
}

func streamerWatchBackedOff(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || streamer.WatchBackoffUntil.IsZero() {
		return false
	}
	return now.Before(streamer.WatchBackoffUntil)
}

func recordMinuteWatchFailure(streamer *entities.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	streamer.WatchFailureCount++
	if streamer.WatchFailureCount < minuteWatchFailureBackoffThreshold {
		return
	}
	multiplier := streamer.WatchFailureCount - minuteWatchFailureBackoffThreshold + 1
	backoff := time.Duration(multiplier) * minuteWatchFailureBackoffBase
	if backoff > minuteWatchFailureBackoffMax {
		backoff = minuteWatchFailureBackoffMax
	}
	streamer.WatchBackoffUntil = now.Add(backoff)
}

func recordMinuteWatchSuccess(streamer *entities.Streamer) {
	if streamer == nil {
		return
	}
	streamer.WatchFailureCount = 0
	streamer.WatchBackoffUntil = time.Time{}
}

func (m *Miner) run(streamers []string, useFollowers bool, order entities.FollowersOrder) {
	m.startedAt = time.Now()
	m.logger.Printf("Twitch Channel Points Miner | v%s", constants.Version)
	m.logger.Println("https://github.com/0x8fv/Twitch-Channel-Points-Miner")
	sessionID := newSessionID()
	m.logger.EmojiEventf(":green_circle:", constants.EventStartup, "Start session: '%s'", sessionID)
	m.stop = make(chan struct{})
	m.initialPoints = make(map[string]int)

	tw, err := classpkg.NewTwitch(m.Username, utils.GetUserAgent("CHROME"), m.Password, m.logger, m.anonymizer)
	if err != nil {
		m.logger.Fatalf("failed to create twitch client: %v", err)
	}
	tw.SetGameChangeHandler(m.handleGameChange)
	m.twitch = tw
	if err := m.twitch.Login(m.Username); err != nil {
		m.logger.Fatalf("login failed: %v", err)
	}
	m.loadWarmStartCache()
	// TODO: Fix Available Campaigns
	// m.logAvailableCampaigns()

	var targets []string
	if useFollowers {
		follows, err := m.twitch.GetFollowers(100, order)
		if err != nil {
			m.logger.Fatalf("failed to load followers: %v", err)
		}
		targets = follows
	} else {
		targets = streamers
	}

	filteredTargets, excludedCount := m.filterExcludedTargets(targets)
	if excludedCount > 0 {
		m.logger.EmojiPrintf(":no_entry_sign:", "Excluded %d streamer(s) via streamers_exclude", excludedCount)
	}
	targets = filteredTargets

	streamerObjs := make([]*entities.Streamer, 0, len(targets))
	loadStartedAt := time.Now()
	m.logger.EmojiPrintf(":hourglass_flowing_sand:", "Loading data for %d streamer(s). Please wait...", len(targets))
	for _, name := range targets {
		if name == "" {
			continue
		}
		settings := m.StreamerSettings
		if override, ok := m.StreamerOverrides[strings.ToLower(name)]; ok {
			settings = override
		}
		s := &entities.Streamer{
			Username:    name,
			Settings:    settings,
			Stream:      entities.NewStream(),
			StreamerURL: fmt.Sprintf("%s/%s", constants.URL, name),
		}
		id, err := m.twitch.GetChannelID(name)
		if err != nil {
			m.logger.Printf("skip %s: %v", m.rawStreamerName(name), err)
			continue
		}
		s.ChannelID = id
		prev := s.ChannelPoints
		if _, err := m.loadChannelPointsContextWithIdentityRepair(s); err != nil {
			m.logger.Printf("context for %s: %v", m.rawStreamerName(name), err)
		} else {
			m.handlePointsUpdate(s, prev, "")
		}
		m.updatePresence(s)
		streamerObjs = append(streamerObjs, s)
		m.initialPoints[streamerInitialPointsKey(s)] = s.ChannelPoints
	}
	m.saveWarmStartCacheSnapshot(streamerObjs)

	if len(streamerObjs) > 0 {
		m.logger.EmojiPrintf(":white_check_mark:", "%d Streamer loaded! (%s)", len(streamerObjs), formatLoadDuration(time.Since(loadStartedAt)))
	}

	claimDropsEnabled := m.shouldClaimDrops(streamerObjs)
	if m.ClaimDropsStartup && claimDropsEnabled {
		m.claimDropsFromInventory("startup drop claim failed")
	}

	m.streamers = streamerObjs

	// ? background loops
	if claimDropsEnabled {
		go m.dropClaimer(m.stop)
	}
	go m.contextRefresher(streamerObjs, m.stop)
	go m.minuteWatcher(streamerObjs, m.stop)
	go m.startPubSub(streamerObjs, m.stop)
	go m.streakRecovery(streamerObjs, m.stop)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	m.shutdown(sessionID)
}

func (m *Miner) dropClaimer(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.claimDropsFromInventory("drop claim failed")
		case <-stop:
			return
		}
	}
}

func (m *Miner) claimDropsFromInventory(errorPrefix string) {
	if m == nil || m.twitch == nil {
		return
	}
	if m.showDropsProgress {
		drops, statuses, err := m.twitch.ClaimAllDropsFromInventoryWithStatuses()
		if err != nil {
			m.logger.Printf("%s: %v", errorPrefix, err)
			return
		}
		m.logDropStatuses(statuses)
		m.logClaimedDrops(drops)
		return
	}
	drops, err := m.twitch.ClaimAllDropsFromInventory()
	if err != nil {
		m.logger.Printf("%s: %v", errorPrefix, err)
		return
	}
	m.logClaimedDrops(drops)
}

func (m *Miner) logClaimedDrops(drops []classpkg.ClaimedDrop) {
	for _, drop := range drops {
		reward := drop.RewardName
		if reward == "" {
			reward = "Drop"
		}
		campaign := drop.CampaignName
		if campaign == "" {
			campaign = "Unknown Campaign"
		}
		progress := formatDropProgress(drop.CurrentValue, drop.RequiredValue)
		percent := progressPercent(drop.CurrentValue, drop.RequiredValue)
		m.logger.EmojiEventf(":package:", constants.EventDropClaim, "Claim %s (%s) %s (%d%%)", reward, campaign, progress, percent)
	}
}

func (m *Miner) logDropStatuses(statuses []classpkg.DropStatus) {
	if m == nil || m.logger == nil || !m.showDropsProgress {
		return
	}
	for _, status := range statuses {
		line := formatDropStatusLine(status)
		if line == "" {
			continue
		}
		m.logger.Eventf(constants.EventDropStatus, "%s", line)
	}
}

func (m *Miner) contextRefresher(streamers []*entities.Streamer, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, s := range streamers {
				prev := s.ChannelPoints
				if _, err := m.loadChannelPointsContextWithIdentityRepair(s); err != nil {
					m.logger.Printf("refresh %s: %v", m.styledStreamerName(s), err)
				} else {
					m.handlePointsUpdate(s, prev, "")
					// TODO: Fix Available Campaigns
					// m.refreshCampaigns(s)
				}
			}
		case <-stop:
			return
		}
	}
}

func (m *Miner) streakRecovery(streamers []*entities.Streamer, stop <-chan struct{}) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()
	for {
		expiringstreaks := 0
		savedstreaks := 0
		for _, s := range streamers {
			if saved, err := m.twitch.RecoverStreak(s); err == nil {
				if saved {
					expiringstreaks += 1
					savedstreaks += 1
				}
			} else {
				expiringstreaks += 1
				m.logger.EmojiPrintf(":ambulance:", "streak recovery %s: %v", m.styledStreamerName(s), err)
			}
		}
		m.logger.EmojiPrintf(":ambulance:", "streak recovery: %d streaks expiring, %d saved", expiringstreaks, savedstreaks)

		select {
		case <-ticker.C:
			continue
		case <-stop:
			return
		}
	}
}

func (m *Miner) minuteWatcher(streamers []*entities.Streamer, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}

		watchList := m.pickStreamersToWatch(streamers)
		if len(watchList) == 0 {
			if m.sleepWithStop(20*time.Second, stop) {
				return
			}
			continue
		}

		interval := m.watchInterval(len(watchList))
		for _, streamer := range watchList {
			select {
			case <-stop:
				return
			default:
			}

			if streamer.Stream != nil && streamer.Stream.LastUpdateAgo() > 10*time.Minute {
				if _, err := m.checkStreamerOnlineWithIdentityRepair(streamer); err != nil {
					m.logger.Printf("online check %s: %v", m.styledStreamerName(streamer), err)
				}
				if !streamer.IsOnline {
					continue
				}
			}

			prevBroadcastID := ""
			prevCreatedAt := time.Time{}
			prevWatchStreakMissing := true
			if streamer.Stream != nil {
				prevBroadcastID = streamer.Stream.BroadcastID
				prevCreatedAt = streamer.Stream.CreatedAt
				prevWatchStreakMissing = streamer.Stream.WatchStreakMissing
			}
			if err := m.sendMinuteWatchedWithIdentityRepair(streamer); err != nil {
				countWatchFailure := true
				if errors.Is(err, classpkg.ErrStreamerOffline) {
					countWatchFailure = false
					live, liveErr := m.twitch.IsStreamLive(streamer.ChannelID)
					if liveErr != nil {
						m.logger.Printf("live check %s: %v", m.styledStreamerName(streamer), liveErr)
					}
					if !live {
						m.setPresence(streamer, false, "minute-watch")
					} else {
						m.logger.Printf("minute watch %s: transient offline response, keeping online", m.styledStreamerName(streamer))
					}
				}
				if countWatchFailure {
					prevFailures := streamer.WatchFailureCount
					recordMinuteWatchFailure(streamer, time.Now())
					if m.logger != nil && prevFailures < minuteWatchFailureBackoffThreshold && streamer.WatchFailureCount >= minuteWatchFailureBackoffThreshold {
						m.logger.Printf("minute watch %s: backing off until %s after repeated failures", m.styledStreamerName(streamer), streamer.WatchBackoffUntil.Format(time.RFC3339))
					}
				}
				m.logger.Errorf("minute watch %s: %v", m.styledStreamerName(streamer), err)
			} else {
				recordMinuteWatchSuccess(streamer)
				m.syncResolvedStreakState(streamer, prevBroadcastID, prevCreatedAt, prevWatchStreakMissing)
				if streamer.Stream != nil && (prevBroadcastID != streamer.Stream.BroadcastID || !prevCreatedAt.Equal(streamer.Stream.CreatedAt) || prevWatchStreakMissing != streamer.Stream.WatchStreakMissing) {
					m.syncWarmStartCacheFromStreamer(streamer)
				}
			}
			m.syncActiveStreakWatch(streamer, time.Now())

			if m.sleepWithStop(interval, stop) {
				return
			}
		}
	}
}

func (m *Miner) refreshStreamForPreference(streamer *entities.Streamer) {
	if streamer == nil || !streamer.IsOnline || m.twitch == nil {
		return
	}
	if streamer.Stream != nil {
		if strings.TrimSpace(streamer.Stream.GameName()) != "" && !streamer.Stream.UpdateRequired() {
			return
		}
	}
	prevBroadcastID := ""
	prevCreatedAt := time.Time{}
	prevWatchStreakMissing := true
	if streamer.Stream != nil {
		prevBroadcastID = streamer.Stream.BroadcastID
		prevCreatedAt = streamer.Stream.CreatedAt
		prevWatchStreakMissing = streamer.Stream.WatchStreakMissing
	}
	if err := m.updateStreamWithIdentityRepair(streamer); err != nil {
		if errors.Is(err, classpkg.ErrStreamerOffline) {
			m.setPresence(streamer, false, "stream-info")
		} else {
			m.logger.Debugf("stream info %s: %v", m.styledStreamerName(streamer), err)
		}
		return
	}
	m.syncResolvedStreakState(streamer, prevBroadcastID, prevCreatedAt, prevWatchStreakMissing)
	if streamer.Stream != nil && (prevBroadcastID != streamer.Stream.BroadcastID || !prevCreatedAt.Equal(streamer.Stream.CreatedAt) || prevWatchStreakMissing != streamer.Stream.WatchStreakMissing) {
		m.syncWarmStartCacheFromStreamer(streamer)
	}
}

func (m *Miner) resolveGameName(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if streamer.Stream != nil {
		if name := strings.TrimSpace(streamer.Stream.GameName()); name != "" {
			return name
		}
	}
	if !m.showGameInfo || m.twitch == nil || !streamer.IsOnline {
		return ""
	}
	prevBroadcastID := ""
	prevCreatedAt := time.Time{}
	prevWatchStreakMissing := true
	if streamer.Stream != nil {
		prevBroadcastID = streamer.Stream.BroadcastID
		prevCreatedAt = streamer.Stream.CreatedAt
		prevWatchStreakMissing = streamer.Stream.WatchStreakMissing
	}
	if err := m.updateStreamWithIdentityRepair(streamer); err != nil {
		if m.logger != nil && m.logger.DebugEnabled() {
			m.logger.Debugf("update stream %s for game: %v", m.styledStreamerName(streamer), err)
		}
		return ""
	}
	m.syncResolvedStreakState(streamer, prevBroadcastID, prevCreatedAt, prevWatchStreakMissing)
	if streamer.Stream != nil && (prevBroadcastID != streamer.Stream.BroadcastID || !prevCreatedAt.Equal(streamer.Stream.CreatedAt) || prevWatchStreakMissing != streamer.Stream.WatchStreakMissing) {
		m.syncWarmStartCacheFromStreamer(streamer)
	}
	if streamer.Stream != nil {
		return strings.TrimSpace(streamer.Stream.GameName())
	}
	return ""
}

func (m *Miner) gameSuffix(streamer *entities.Streamer) string {
	name := m.gameInfo(streamer)
	if name != "" {
		return name
	}
	return ""
}

// func (m *Miner) gameInfo(streamer *entities.Streamer) (string, bool) {
func (m *Miner) gameInfo(streamer *entities.Streamer) string {
	if m == nil || !m.showGameInfo {
		return ""
	}
	// hasDrops := m.showDropsIndicator && m.streamHasDrops(streamer)
	// if !m.showGameInfo {
	// 	return "", hasDrops
	// }
	// return m.resolveGameName(streamer), hasDrops
	return m.resolveGameName(streamer)
}

func (m *Miner) watchContext(streamer *entities.Streamer) string {
	// name, hasDrops := m.gameInfo(streamer)
	name := m.gameInfo(streamer)
	if name != "" {
		label := fmt.Sprintf("| %sGame:%s", colorGameLabel, colorReset)
		// if hasDrops {
		// 	return fmt.Sprintf("%s %s %s", label, name, m.dropIndicator())
		// }
		return fmt.Sprintf("%s %s", label, name)
	}
	// if hasDrops {
	// 	return m.dropIndicator()
	// }
	return ""
}

// TODO: Fix Available Campaigns
// func (m *Miner) dropIndicator() string {
// 	if !m.showDropsIndicator {
// 		return ""
// 	}
// 	return fmt.Sprintf("%s(DROPS)%s", colorDropsAccent, colorReset)
// }

// func (m *Miner) streamHasDrops(streamer *entities.Streamer) bool {
// 	if streamer == nil || streamer.Stream == nil {
// 		return false
// 	}
// 	return m.twitch != nil && m.twitch.GameHasActiveDrops(streamer.Stream)
// }

// func (m *Miner) refreshCampaigns(streamer *entities.Streamer) {
// 	if streamer == nil || streamer.Stream == nil || m.twitch == nil {
// 		return
// 	}
// 	if !(streamer.Settings.ClaimDrops || m.showDropsIndicator) {
// 		return
// 	}
// 	campaigns, hasDrops, err := m.twitch.CampaignIDsForStreamer(streamer)
// 	if err != nil {
// 		if m.logger != nil && m.logger.DebugEnabled() {
// 			m.logger.Debugf("campaigns for %s: %v", streamer.Username, err)
// 		}
// 		return
// 	}
// 	streamer.Stream.CampaignIDs = campaigns
// 	streamer.Stream.CampaignsResolved = true
// 	streamer.Stream.DropsActive = hasDrops || m.twitch.GameHasActiveDrops(streamer.Stream)
// }

//	func (m *Miner) logAvailableCampaigns() {
//		if m.logger == nil || m.twitch == nil {
//			return
//		}
//		summaries := m.twitch.AvailableCampaignSummaries()
//		if len(summaries) == 0 {
//			m.logger.Printf("Active drop campaigns: none detected")
//			return
//		}
//		m.logger.Printf("Active drop campaigns (%d):", len(summaries))
//		for _, summary := range summaries {
//			m.logger.Printf(" - %s", summary)
//		}
//	}
func (m *Miner) gamePreference(streamer *entities.Streamer) (int, bool) {
	baseRank := len(m.gamePriority) + 1
	if streamer == nil || streamer.Stream == nil {
		return baseRank, false
	}
	name := strings.ToLower(strings.TrimSpace(streamer.Stream.GameName()))
	if name == "" {
		return baseRank, false
	}
	if _, ok := m.gameExclusions[name]; ok {
		return 0, true
	}
	if idx, ok := m.gamePriorityIndex[name]; ok {
		return idx, false
	}
	return baseRank, false
}

func (m *Miner) resolveTimedOutStreak(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if !streamer.Settings.WatchStreak || !streamer.Stream.WatchStreakMissing {
		return false
	}
	if streamer.Stream.MinuteWatched < m.streakPriorityLimit(now) {
		return false
	}
	streamer.Stream.WatchStreakMissing = false
	return true
}

func (m *Miner) pickStreamersToWatch(streamers []*entities.Streamer) []*entities.Streamer {
	now := time.Now()
	type candidate struct {
		idx           int
		rank          int
		game          string
		position      int
		priorityGame  bool
		isStreakReady bool
	}
	candidates := make([]candidate, 0, len(streamers))
	candidateByIdx := make(map[int]candidate, len(streamers))
	streakCandidates := make([]candidate, 0, len(streamers))
	streakIdx := make(map[int]struct{})
	hasPriorityGameStreak := false
	for idx, s := range streamers {
		if s == nil || !s.IsOnline {
			continue
		}
		if streamerWatchBackedOff(s, now) {
			continue
		}
		if !s.OnlineAt.IsZero() && now.Sub(s.OnlineAt) < 30*time.Second {
			continue
		}
		m.refreshStreamForPreference(s)
		if s == nil || !s.IsOnline {
			continue
		}
		if m.resolveTimedOutStreak(s, now) {
			m.syncWarmStartCacheFromStreamer(s)
		}
		rank, excluded := m.gamePreference(s)
		if excluded {
			continue
		}
		game := ""
		if s.Stream != nil {
			game = strings.ToLower(strings.TrimSpace(s.Stream.GameName()))
		}
		_, priorityGame := m.gamePriorityIndex[game]
		isStreak := m.shouldPrioritizeStreak(s, now)
		cand := candidate{
			idx:           idx,
			rank:          rank,
			game:          game,
			position:      len(candidates),
			priorityGame:  priorityGame,
			isStreakReady: isStreak,
		}
		candidates = append(candidates, cand)
		candidateByIdx[idx] = cand
		if isStreak {
			streakCandidates = append(streakCandidates, cand)
			streakIdx[idx] = struct{}{}
			if priorityGame {
				hasPriorityGameStreak = true
			}
		}
	}

	sortCandidates := func(list []candidate, less func(a, b candidate) bool, includeGameRank bool) []candidate {
		out := append([]candidate(nil), list...)
		sort.SliceStable(out, func(i, j int) bool {
			a, b := out[i], out[j]
			if less != nil {
				ai := less(a, b)
				aj := less(b, a)
				if ai && !aj {
					return true
				}
				if aj && !ai {
					return false
				}
			}
			if includeGameRank && a.rank != b.rank {
				return a.rank < b.rank
			}
			return a.position < b.position
		})
		return out
	}

	selected := make([]int, 0, maxConcurrentWatchers)
	seen := make(map[int]struct{})
	selectedGames := make(map[string]struct{})
	selectedReason := make(map[int]string)
	add := func(c candidate, reason string, force bool) {
		if len(selected) >= maxConcurrentWatchers {
			return
		}
		if _, ok := seen[c.idx]; ok {
			return
		}
		game := c.game
		if !force && game != "" {
			if _, ok := selectedGames[game]; ok {
				otherAvailable := false
				for _, alt := range candidates {
					if _, picked := seen[alt.idx]; picked {
						continue
					}
					if alt.game != "" && alt.game != game {
						otherAvailable = true
						break
					}
				}
				if otherAvailable {
					return
				}
			}
		}
		seen[c.idx] = struct{}{}
		if game != "" {
			selectedGames[game] = struct{}{}
		}
		selected = append(selected, c.idx)
		if reason != "" {
			selectedReason[c.idx] = reason
		}
	}

	pick := func(list []candidate, includeGameRank bool, less func(a, b candidate) bool, reason string, force bool) {
		for _, c := range sortCandidates(list, less, includeGameRank) {
			add(c, reason, force)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}

	m.activeStreakMu.Lock()
	if len(m.activeStreakWatches) > 0 {
		activeStreaks := make([]candidate, 0, len(m.activeStreakWatches))
		for _, c := range candidates {
			s := streamers[c.idx]
			if s == nil {
				continue
			}
			key := m.activeStreakWatchKey(s)
			watch, ok := m.activeStreakWatches[key]
			if !ok {
				continue
			}
			if !m.shouldKeepActiveStreak(s, now) || !activeStreakWatchMatches(s, watch) {
				delete(m.activeStreakWatches, key)
				continue
			}
			activeStreaks = append(activeStreaks, c)
		}
		for _, c := range sortCandidates(activeStreaks, nil, false) {
			add(c, "ACTIVE_STREAK", true)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}
	m.activeStreakMu.Unlock()

	skipEarlyStreak := len(m.gamePriority) > 0 && !hasPriorityGameStreak

	for _, priority := range m.watchPriorities {
		if len(selected) >= maxConcurrentWatchers {
			break
		}
		switch priority {
		case watchPriorityOrder:
			pick(candidates, false, nil, "ORDER", false)
		case watchPriorityStreak:
			if skipEarlyStreak {
				continue
			}
			streaks := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				if m.shouldPrioritizeStreak(streamers[c.idx], now) {
					streaks = append(streaks, c)
				}
			}
			pick(streaks, true, nil, "STREAK", true)
		case watchPriorityDrops:
			drops := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				s := streamers[c.idx]
				if s == nil || s.Stream == nil {
					continue
				}
				// if s.Settings.ClaimDrops && m.streamHasDrops(s) {
				if s.Settings.ClaimDrops {
					drops = append(drops, c)
				}
			}
			pick(drops, true, nil, "DROPS", false)
		case watchPrioritySubscribed:
			subscribed := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				s := streamers[c.idx]
				if s == nil {
					continue
				}
				if s.HasActiveMultipliers() {
					subscribed = append(subscribed, c)
				}
			}
			pick(subscribed, true, func(a, b candidate) bool {
				return streamers[a.idx].TotalMultiplier() > streamers[b.idx].TotalMultiplier()
			}, "SUBSCRIBED", false)
		case watchPriorityPointsAscending:
			asc := append([]candidate(nil), candidates...)
			pick(asc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints < streamers[b.idx].ChannelPoints
			}, "POINTS_ASC", false)
		case watchPriorityPointsDescending:
			desc := append([]candidate(nil), candidates...)
			pick(desc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints > streamers[b.idx].ChannelPoints
			}, "POINTS_DESC", false)
		}
	}

	hasStreakSelected := false
	for _, idx := range selected {
		if _, ok := streakIdx[idx]; ok {
			hasStreakSelected = true
			break
		}
	}
	if !hasStreakSelected && len(streakCandidates) > 0 && len(selected) > 0 {
		var streakPick *candidate
		for _, c := range sortCandidates(streakCandidates, nil, true) {
			if _, ok := seen[c.idx]; ok {
				continue
			}
			streakPick = &c
			break
		}
		if streakPick != nil {
			if len(selected) < maxConcurrentWatchers {
				add(*streakPick, "FORCE_STREAK_SLOT2", true)
			} else {
				keepIdx := selected[0]
				selected = selected[:0]
				seen = make(map[int]struct{})
				selectedGames = make(map[string]struct{})
				if keepCand, ok := candidateByIdx[keepIdx]; ok {
					add(keepCand, selectedReason[keepIdx], false)
				}
				if len(selected) < maxConcurrentWatchers {
					if _, ok := seen[streakPick.idx]; !ok {
						seen[streakPick.idx] = struct{}{}
						if streakPick.game != "" {
							selectedGames[streakPick.game] = struct{}{}
						}
						selected = append(selected, streakPick.idx)
						selectedReason[streakPick.idx] = "FORCE_STREAK_SLOT2"
					}
				}
			}
		}
	}

	if skipEarlyStreak && len(selected) >= 2 {
		first := candidateByIdx[selected[0]]
		second := candidateByIdx[selected[1]]
		if first.isStreakReady && !first.priorityGame && (!second.isStreakReady || second.priorityGame) {
			selected[0], selected[1] = selected[1], selected[0]
		}
	}

	if len(selected) < maxConcurrentWatchers {
		pick(candidates, true, nil, "FALLBACK", false)
	}

	watchList := make([]*entities.Streamer, 0, len(selected))
	for _, idx := range selected {
		watchList = append(watchList, streamers[idx])
		m.syncActiveStreakWatch(streamers[idx], now)
	}

	if m.logger != nil && m.logWatchQueue {
		interval := m.watchInterval(len(selected))
		lines := make([]string, 0, len(selected)+1)
		lines = append(lines, fmt.Sprintf("WATCH queue (≈%s between streamers):", formatDuration(interval)))
		for slot, idx := range selected {
			s := streamers[idx]
			cand := candidateByIdx[idx]
			reason := selectedReason[idx]
			if reason == "" {
				reason = "UNKNOWN"
			}
			streakRemain := ""
			if s != nil && s.Stream != nil && s.Stream.WatchStreakMissing {
				remainingMinutes := m.streakPriorityLimit(now) - s.Stream.MinuteWatched
				if remainingMinutes < 0 {
					remainingMinutes = 0
				}
				remaining := time.Duration(remainingMinutes * float64(time.Minute))
				streakRemain = fmt.Sprintf(", streakRemaining=%s", formatDuration(remaining))
			}
			detail := fmt.Sprintf(
				"%s (reason=%s, streak=%t, priorityGame=%t, rank=%d, pos=%d%s)",
				m.styledStreamerName(s),
				reason,
				cand.isStreakReady,
				cand.priorityGame,
				cand.rank,
				cand.position,
				streakRemain,
			)
			lines = append(lines, fmt.Sprintf("SLOT %d: %s", slot+1, detail))
		}
		m.logger.Printf(strings.Join(lines, "\n"))
	}

	return watchList
}

func (m *Miner) streakCooldownBlocksCurrentStream(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || streamer.OfflineAt.IsZero() {
		return false
	}
	if now.Sub(streamer.OfflineAt) > 30*time.Minute {
		return false
	}
	if streamer.Stream == nil {
		return true
	}
	if streamStartSurvivesRecentOffline(streamer.Stream.StreamUpAt, streamer.OfflineAt) {
		return false
	}
	if streamStartSurvivesRecentOffline(streamer.Stream.CreatedAt, streamer.OfflineAt) {
		return false
	}
	return true
}

func streamStartSurvivesRecentOffline(start, offlineAt time.Time) bool {
	if start.IsZero() || offlineAt.IsZero() {
		return false
	}
	if !start.Before(offlineAt) {
		return true
	}
	return offlineAt.Sub(start) <= falseOfflineStreamStartGrace
}

func (m *Miner) shouldPrioritizeStreak(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if !streamer.Settings.WatchStreak || activeResolvedStreakCarryover(streamer, now) || !streamer.Stream.WatchStreakMissing {
		return false
	}
	if m.streakCooldownBlocksCurrentStream(streamer, now) {
		return false
	}
	// ? Keep streak priority long enough for Twitch to issue the streak check (typically ~15 minutes).
	return streamer.Stream.MinuteWatched < m.streakPriorityLimit(now)
}

func resolvedStreakCarryoverExpiry(streamer *entities.Streamer, now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	base := now
	if streamer != nil && !streamer.OfflineAt.IsZero() && !streamer.OfflineAt.After(now) && now.Sub(streamer.OfflineAt) <= resolvedStreakCarryoverWindow {
		base = streamer.OfflineAt
	}
	return base.Add(resolvedStreakCarryoverWindow)
}

func rememberResolvedStreakCarryover(streamer *entities.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if streamer.CompletedWatchStreak || activeResolvedStreakCarryover(streamer, now) {
		activateResolvedStreakCarryover(streamer, now)
		return
	}
	streamer.ResolvedStreakCarryover = false
	streamer.ResolvedStreakCarryoverUntil = time.Time{}
}

func restoreResolvedStreakCarryover(streamer *entities.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	if !activeResolvedStreakCarryover(streamer, now) {
		streamer.ResolvedStreakCarryover = false
		streamer.ResolvedStreakCarryoverUntil = time.Time{}
		streamer.CompletedWatchStreak = false
		return
	}
	streamer.CompletedWatchStreak = true
	applyResolvedStreakCarryover(streamer, now)
}

func activeResolvedStreakCarryover(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || !streamer.ResolvedStreakCarryover || streamer.ResolvedStreakCarryoverUntil.IsZero() {
		return false
	}
	if now.After(streamer.ResolvedStreakCarryoverUntil) {
		streamer.ResolvedStreakCarryover = false
		streamer.ResolvedStreakCarryoverUntil = time.Time{}
		return false
	}
	return true
}

func applyResolvedStreakCarryover(streamer *entities.Streamer, now time.Time) {
	if streamer == nil || streamer.Stream == nil || !activeResolvedStreakCarryover(streamer, now) {
		return
	}
	streamer.Stream.WatchStreakMissing = false
}

func activateResolvedStreakCarryover(streamer *entities.Streamer, now time.Time) {
	if streamer == nil {
		return
	}
	streamer.ResolvedStreakCarryover = true
	streamer.ResolvedStreakCarryoverUntil = resolvedStreakCarryoverExpiry(streamer, now)
}

func markActualStreakCompleted(streamer *entities.Streamer) {
	if streamer == nil {
		return
	}
	streamer.CompletedWatchStreak = true
	if streamer.Stream != nil {
		streamer.Stream.WatchStreakMissing = false
	}
}

func (m *Miner) syncResolvedStreakState(streamer *entities.Streamer, prevBroadcastID string, prevCreatedAt time.Time, prevWatchStreakMissing bool) {
	if streamer == nil || streamer.Stream == nil {
		return
	}
	if prevWatchStreakMissing && !streamer.Stream.WatchStreakMissing {
		streamer.CompletedWatchStreak = true
	}
	segmentRolled := (prevBroadcastID != "" && streamer.Stream.BroadcastID != "" && prevBroadcastID != streamer.Stream.BroadcastID) ||
		(!prevCreatedAt.IsZero() && !streamer.Stream.CreatedAt.IsZero() && !prevCreatedAt.Equal(streamer.Stream.CreatedAt))
	now := time.Now()
	if segmentRolled && streamer.CompletedWatchStreak {
		activateResolvedStreakCarryover(streamer, now)
	}
	applyResolvedStreakCarryover(streamer, now)
}

func (m *Miner) activeStreakWatchKey(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if channelID := strings.TrimSpace(streamer.ChannelID); channelID != "" {
		return channelID
	}
	return strings.ToLower(strings.TrimSpace(streamer.Username))
}

func activeStreakWatchMatches(streamer *entities.Streamer, watch activeStreakWatch) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if streamer.Stream.BroadcastID != "" || watch.BroadcastID != "" {
		return streamer.Stream.BroadcastID != "" && streamer.Stream.BroadcastID == watch.BroadcastID
	}
	if streamer.Stream.CreatedAt.IsZero() || watch.CreatedAt.IsZero() {
		return false
	}
	return streamer.Stream.CreatedAt.Equal(watch.CreatedAt)
}

func (m *Miner) shouldKeepActiveStreak(streamer *entities.Streamer, now time.Time) bool {
	if !m.shouldPrioritizeStreak(streamer, now) {
		return false
	}
	return streamer.Stream.MinuteWatched > 0
}

func (m *Miner) syncActiveStreakWatch(streamer *entities.Streamer, now time.Time) {
	if m == nil {
		return
	}
	key := m.activeStreakWatchKey(streamer)
	if key == "" {
		return
	}
	if !m.shouldKeepActiveStreak(streamer, now) {
		m.activeStreakMu.Lock()
		if m.activeStreakWatches != nil {
			delete(m.activeStreakWatches, key)
		}
		m.activeStreakMu.Unlock()
		return
	}
	m.activeStreakMu.Lock()
	if m.activeStreakWatches == nil {
		m.activeStreakWatches = make(map[string]activeStreakWatch)
	}
	m.activeStreakWatches[key] = activeStreakWatch{
		BroadcastID: streamer.Stream.BroadcastID,
		CreatedAt:   streamer.Stream.CreatedAt,
	}
	m.activeStreakMu.Unlock()
}

// ? streakPriorityLimit adjusts streak priority duration:
// ? - default 7 minutes
// ? - extended to 20 minutes after 10 hours runtime to avoid churn late in long sessions.
func (m *Miner) streakPriorityLimit(now time.Time) float64 {
	if m == nil {
		return streakPriorityMinutesBase
	}
	if m.startedAt.IsZero() {
		return streakPriorityMinutesBase
	}
	if now.Sub(m.startedAt) > 10*time.Hour {
		return streakPriorityMinutesExtended
	}
	return streakPriorityMinutesBase
}

func (m *Miner) watchInterval(count int) time.Duration {
	if count <= 0 {
		return 20 * time.Second
	}
	interval := time.Duration(float64(20*time.Second) / float64(count))
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	return interval
}

func (m *Miner) sleepWithStop(duration time.Duration, stop <-chan struct{}) bool {
	if duration <= 0 {
		return false
	}
	const chunks = 3
	step := duration / chunks
	if step <= 0 {
		step = duration
	}
	elapsed := time.Duration(0)
	for elapsed < duration {
		remaining := duration - elapsed
		if remaining < step {
			step = remaining
		}
		timer := time.NewTimer(step)
		select {
		case <-stop:
			timer.Stop()
			return true
		case <-timer.C:
		}
		elapsed += step
	}
	return false
}

func (m *Miner) startPubSub(streamers []*entities.Streamer, stop <-chan struct{}) {
	client := classpkg.NewPubSubClient(
		m.twitch,
		m.logger,
		m.anonymizer,
		streamers,
		m.DisableSSLCertVerification,
		m.handlePubSubGain,
		m.handlePubSubPresence,
	)
	client.Start(stop)
}

func (m *Miner) shutdown(sessionID string) {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	fmt.Println()
	fmt.Println()
	fmt.Println()
	m.logger.EmojiEventf(":stop_sign:", constants.EventShutdown, "Ending session: '%s'", sessionID)
	duration := formatDuration(time.Since(m.startedAt))
	m.logger.EmojiPrintf(":hourglass:", "Duration %s", duration)
	totalPointsChange := 0
	for _, s := range m.streamers {
		totalPointsChange += s.ChannelPoints - m.initialPoints[streamerInitialPointsKey(s)]
	}
	if m.anonymizeLogs() {
		m.logger.EmojiPrintf(":chart_with_upwards_trend:", "Total Points gained: [hidden]")
	} else {
		totalSign := "+"
		totalColor := colorGreen
		if totalPointsChange < 0 {
			totalPointsChange = -totalPointsChange
			totalSign = "-"
			totalColor = colorRed
		}
		m.logger.EmojiPrintf(":chart_with_upwards_trend:", "Total Points gained: %s%s%d%s", totalColor, totalSign, totalPointsChange, colorReset)
	}
	for _, s := range m.streamers {
		initial := m.initialPoints[streamerInitialPointsKey(s)]
		total := s.ChannelPoints - initial
		if total == 0 && len(s.History) == 0 {
			continue
		}
		signColor := colorGreen
		sign := "+"
		if total < 0 {
			signColor = colorRed
			sign = "-"
			total = -total
		}
		points := m.formattedStreamerPoints(s)
		if m.anonymizeLogs() {
			m.logger.EmojiPrintf(":moneybag:", "%s (%s%s%s points), Total Points [hidden]", m.styledStreamerName(s), colorCyan, points, colorReset)
			if s.History != nil {
				for reason, entry := range s.History {
					m.logger.Printf("                         %s (%d times, [hidden])", reason, entry.Count)
				}
			}
		} else {
			m.logger.EmojiPrintf(":moneybag:", "%s (%s%s%s points), Total Points %s%s%d%s", m.styledStreamerName(s), colorCyan, points, colorReset, signColor, sign, total, colorReset)
			if s.History != nil {
				for reason, entry := range s.History {
					m.logger.Printf("                         %s (%d times, %d gained)", reason, entry.Count, entry.Amount)
				}
			}
		}
	}
	m.stopAllChatWatchers()
	os.Exit(0)
}

func (m *Miner) updatePresence(streamer *entities.Streamer) {
	online, err := m.checkStreamerOnlineWithIdentityRepair(streamer)
	if err != nil {
		m.logger.Printf("online check %s: %v", m.styledStreamerName(streamer), err)
		return
	}
	if online && !streamer.PresenceKnown {
		m.applyWarmStartCache(streamer)
	}
	m.setPresence(streamer, online, "poll")
}

func (m *Miner) logOnline(streamer *entities.Streamer) {
	name := m.styledStreamerName(streamer)
	points := m.formattedStreamerPoints(streamer)
	gameSuffix := ""
	if suffix := m.gameSuffix(streamer); suffix != "" {
		gameSuffix = fmt.Sprintf(" | %s %s", fmt.Sprintf("%sPlaying:%s", colorGameLabel, colorReset), suffix)
	}
	streaklengthSuffix := ""
	if m.twitch != nil && streamer != nil && streamer.ChannelID != "" {
		streaklengthSuffix = fmt.Sprintf(" | streak length %d", m.twitch.RewardListStreakLength(streamer.ChannelID))
	}
	m.logger.EmojiEventf(":partying_face:", constants.EventStreamerOnline, "%s (%s%s%s points) is %sOnline%s!%s%s", name, colorCyan, points, colorReset, colorGreen, colorReset, streaklengthSuffix, gameSuffix)
}

func (m *Miner) logOffline(streamer *entities.Streamer) {
	name := m.styledStreamerName(streamer)
	points := m.formattedStreamerPoints(streamer)
	streaklengthSuffix := ""
	if m.twitch != nil && streamer != nil && streamer.ChannelID != "" {
		streaklengthSuffix = fmt.Sprintf(" | streak length %d", m.twitch.RewardListStreakLength(streamer.ChannelID))
	}
	m.logger.EmojiEventf(":sleeping:", constants.EventStreamerOffline, "%s (%s%s%s points) is %sOffline%s!%s", name, colorCyan, points, colorReset, colorRed, colorReset, streaklengthSuffix)
}

func (m *Miner) handleGameChange(streamer *entities.Streamer, previous, current string) {
	if m == nil || m.logger == nil || !m.showGameInfo {
		return
	}
	current = strings.TrimSpace(current)
	previous = strings.TrimSpace(previous)
	// ? Skip initial load while bringing a streamer online to avoid duplicate "now playing" spam.
	if current == "" || previous == "" || strings.EqualFold(previous, current) {
		return
	}
	m.logger.EmojiPrintf(":video_game:", "%s now playing: %s!", m.styledStreamerName(streamer), current)
}

func displayName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func (m *Miner) anonymizeLogs() bool {
	return m != nil && m.anonymizer != nil && m.anonymizer.Enabled()
}

func (m *Miner) rawStreamerName(raw string) string {
	if m.anonymizeLogs() {
		return m.anonymizer.Name(raw)
	}
	return raw
}

func (m *Miner) styledStreamerName(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	name := displayName(streamer.Username)
	if m.anonymizeLogs() {
		name = m.anonymizer.StreamerName(streamer)
	}
	if streamer.HasActiveMultipliers() {
		return fmt.Sprintf("%s%s%s", colorDropsAccent, name, colorReset)
	}
	return name
}

func (m *Miner) formattedStreamerPoints(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	points := streamer.ChannelPoints
	if m.anonymizeLogs() {
		points = m.anonymizer.PseudoChannelPoints(streamer)
	}
	return formatChannelPoints(points)
}

func formatChannelPoints(points int) string {
	value := points
	if value < 0 {
		value = -value
	}
	switch {
	case value >= 1_000_000:
		return formatPointsWithSuffix(value, 1_000_000, "M")
	case value >= 1_000:
		return formatPointsWithSuffix(value, 1_000, "k")
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatPointsWithSuffix(points int, divisor float64, suffix string) string {
	short := float64(points) / divisor
	formatted := fmt.Sprintf("%.2f", short)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	return formatted + suffix
}

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

func formatDropStatusLine(status classpkg.DropStatus) string {
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

func dropStatusLabel(status classpkg.DropStatus) string {
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

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	d = d.Round(time.Second)
	day := 24 * time.Hour
	days := d / day
	d -= days * day
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%02dh", hours))
	}
	if minutes > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%02dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%02ds", seconds))
	return strings.Join(parts, " ")
}

func (m *Miner) handlePointsUpdate(streamer *entities.Streamer, previous int, reason string) {
	if !streamer.PointsInit {
		streamer.PointsInit = true
		return
	}
	delta := streamer.ChannelPoints - previous
	m.logPointsDelta(streamer, delta, reason)
}

func (m *Miner) logPointsDelta(streamer *entities.Streamer, delta int, reason string) {
	if delta == 0 {
		return
	}
	name := m.styledStreamerName(streamer)
	points := m.formattedStreamerPoints(streamer)
	sign := "+"
	valueColor := colorGreen
	if delta < 0 {
		sign = "-"
		delta = -delta
		valueColor = colorRed
	}
	if reason == "" {
		return
	}
	reasonDisplay := reason
	if reason == "WATCH" {
		if ctx := m.watchContext(streamer); ctx != "" {
			reasonDisplay = fmt.Sprintf("%s %s", reason, ctx)
		}
	}
	event := constants.EventFromGainReason(reason)
	if event != "" {
		m.logger.EmojiEventf(
			":rocket:",
			event,
			"%s%s%d%s → %s (%s%s%s points) - Reason: %s",
			valueColor,
			sign,
			delta,
			colorReset,
			name,
			colorCyan,
			points,
			colorReset,
			reasonDisplay,
		)
	} else {
		m.logger.EmojiPrintf(
			":rocket:",
			"%s%s%d%s → %s (%s%s%s points) - Reason: %s",
			valueColor,
			sign,
			delta,
			colorReset,
			name,
			colorCyan,
			points,
			colorReset,
			reasonDisplay,
		)
	}
}

func (m *Miner) handlePubSubGain(streamer *entities.Streamer, earned int, reason string, balance int) {
	prev := streamer.ChannelPoints
	expected := prev + earned
	prevWatchStreakMissing := true
	if streamer != nil && streamer.Stream != nil {
		prevWatchStreakMissing = streamer.Stream.WatchStreakMissing
	}

	// ? Prefer applying the delta (`earned`) over trusting the absolute balance from PubSub
	// ? PubSub messages can arrive out of order, and may contain a stale pre-spend balance
	// ? (e.g. after placing a prediction bet), which would otherwise incorrectly inflate the local state
	newBalance := expected
	if earned == 0 && balance != 0 {
		newBalance = balance
	}

	if newBalance < 0 {
		newBalance = 0
	}
	// ? For positive earn events, keep the balance monotonic to avoid logging negative deltas
	// ? when older balances arrive after a streak of gains.
	if earned >= 0 && newBalance < prev {
		newBalance = prev
	}

	streamer.ChannelPoints = newBalance
	if !streamer.PointsInit {
		streamer.PointsInit = true
	}
	delta := earned
	if delta == 0 {
		delta = streamer.ChannelPoints - prev
	}
	m.logPointsDelta(streamer, delta, reason)
	m.updateHistory(streamer, reason, earned)
	if streamer != nil && streamer.Stream != nil && prevWatchStreakMissing != streamer.Stream.WatchStreakMissing {
		m.syncWarmStartCacheFromStreamer(streamer)
	}
}

func (m *Miner) updateHistory(streamer *entities.Streamer, reason string, amount int) {
	if reason == "" {
		return
	}
	streamer.HistoryMu.Lock()
	if streamer.History == nil {
		streamer.History = make(map[string]*entities.HistoryEntry)
	}
	entry, ok := streamer.History[reason]
	if !ok {
		entry = &entities.HistoryEntry{}
		streamer.History[reason] = entry
	}
	entry.Count++
	entry.Amount += amount
	streamer.HistoryMu.Unlock()
	if streamer.Stream == nil {
		return
	}
	if reason == "WATCH" {
		streamer.Stream.WatchCount++
		if streamer.Stream.WatchStreakMissing && streamer.Stream.WatchCount >= 1 {
			markActualStreakCompleted(streamer)
		}
		m.syncActiveStreakWatch(streamer, time.Now())
		return
	}
	if reason == "WATCH_STREAK" {
		markActualStreakCompleted(streamer)
	}
	m.syncActiveStreakWatch(streamer, time.Now())
}

func (m *Miner) handlePubSubPresence(streamer *entities.Streamer, online bool, reason string) {
	m.setPresence(streamer, online, fmt.Sprintf("pubsub:%s", reason))
}

func (m *Miner) setPresence(streamer *entities.Streamer, online bool, reason string) {
	prevKnown := streamer.PresenceKnown
	prevOnline := streamer.IsOnline
	now := time.Now()
	streamer.PresenceKnown = true
	if online != prevOnline || !prevKnown {
		if online {
			streamer.OnlineAt = now
		} else {
			streamer.OfflineAt = now
		}
	}
	if prevKnown && prevOnline != online {
		if online {
			restoreResolvedStreakCarryover(streamer, now)
		} else {
			rememberResolvedStreakCarryover(streamer, now)
		}
	}
	streamer.IsOnline = online
	m.syncActiveStreakWatch(streamer, now)
	m.updateChatPresence(streamer, online)
	if online && m.showGameInfo {
		m.resolveGameName(streamer)
	}
	if !prevKnown {
		if online {
			m.logOnline(streamer)
		} else {
			m.logOffline(streamer)
		}
		m.syncWarmStartCacheFromStreamer(streamer)
		return
	}
	if prevOnline != online {
		if online {
			m.logOnline(streamer)
		} else {
			m.logOffline(streamer)
		}
		m.syncWarmStartCacheFromStreamer(streamer)
		return
	}
	if reason != "" && !online {
		// ? Offline message already logged for state changes; keep silent on no-op toggles.
		m.syncWarmStartCacheFromStreamer(streamer)
		return
	}
	m.syncWarmStartCacheFromStreamer(streamer)
}

func (m *Miner) updateChatPresence(streamer *entities.Streamer, online bool) {
	if streamer == nil {
		return
	}
	if shouldJoinChat(streamer.Settings.IRCMode, online) {
		m.startChatWatcher(streamer)
		return
	}
	m.stopChatWatcher(streamer)
}

func (m *Miner) startChatWatcher(streamer *entities.Streamer) {
	if streamer == nil || m.twitch == nil {
		return
	}
	token := m.twitch.ChatToken()
	if token == "" {
		return
	}
	key := strings.ToLower(streamer.Username)
	m.chatMu.Lock()
	if _, exists := m.chatWatchers[key]; exists {
		m.chatMu.Unlock()
		return
	}
	watcher := classpkg.NewChatClient(m.Username, token, streamer.Username, m.logger, m.disableAtInNickname, m.anonymizer)
	m.chatWatchers[key] = watcher
	m.chatMu.Unlock()
	if m.logger != nil {
		m.logger.EmojiPrintf(":speech_balloon:", "Join IRC Chat: %s", m.styledStreamerName(streamer))
	}
	watcher.Start()
}

func (m *Miner) stopChatWatcher(streamer *entities.Streamer) {
	if streamer == nil {
		return
	}
	key := strings.ToLower(streamer.Username)
	m.chatMu.Lock()
	watcher, ok := m.chatWatchers[key]
	if ok {
		delete(m.chatWatchers, key)
	}
	m.chatMu.Unlock()
	if ok && watcher != nil {
		if m.logger != nil {
			m.logger.EmojiPrintf(":speech_balloon:", "Leave IRC Chat: %s", m.styledStreamerName(streamer))
		}
		watcher.Stop()
	}
}

func (m *Miner) stopAllChatWatchers() {
	m.chatMu.Lock()
	watchers := make([]*classpkg.ChatClient, 0, len(m.chatWatchers))
	for key, watcher := range m.chatWatchers {
		if watcher != nil {
			watchers = append(watchers, watcher)
		}
		delete(m.chatWatchers, key)
	}
	m.chatMu.Unlock()
	for _, watcher := range watchers {
		watcher.Stop()
	}
}

func shouldJoinChat(mode entities.IRCMode, online bool) bool {
	switch mode {
	case entities.IRCModeAlways:
		return true
	case entities.IRCModeNever:
		return false
	case entities.IRCModeOffline:
		return !online
	case entities.IRCModeOnline:
		return online
	default:
		return online
	}
}

func formatLoadDuration(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.1f seconds", d.Seconds())
}

// ? newSessionID creates a UUID-like string for session logging.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// ? variant and version bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
