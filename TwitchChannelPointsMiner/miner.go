package twitchchannelpointsminer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
)

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
	showGameInfo               bool
	logWatchQueue              bool
	anonymizer                 *privacy.Anonymizer
	// showDropsIndicator         bool
}

func NewMiner(username, password string, claimDropsStartup bool, disableCertCheck bool, loggerSettings LoggerSettings, streamerSettings entities.StreamerSettings, streamerOverrides map[string]entities.StreamerSettings, priorityNames []string, streamerExclude []string, gamePriority []string, gameExclude []string, showGameInfo bool, logWatchQueue bool) *Miner {
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
	return &Miner{
		Username:                   username,
		Password:                   password,
		ClaimDropsStartup:          claimDropsStartup,
		DisableSSLCertVerification: disableCertCheck,
		LoggerSettings:             loggerSettings,
		StreamerSettings:           streamerSettings,
		StreamerOverrides:          streamerOverrides,
		logger:                     NewLogger(loggerSettings, username),
		watchPriorities:            parseWatchPriorities(priorityNames),
		streamerExclusions:         excludedStreamers,
		gamePriority:               priorityList,
		gamePriorityIndex:          priorityIndex,
		gameExclusions:             excludedGames,
		chatWatchers:               make(map[string]*classpkg.ChatClient),
		showGameInfo:               showGameInfo,
		logWatchQueue:              logWatchQueue,
		anonymizer:                 privacy.New(loggerSettings.AnonymizeLogs),
		// showDropsIndicator:         showDropsIndicator,
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
		if _, err := m.twitch.LoadChannelPointsContext(s); err != nil {
			m.logger.Printf("context for %s: %v", m.rawStreamerName(name), err)
		} else {
			m.handlePointsUpdate(s, prev, "")
		}
		m.updatePresence(s)
		streamerObjs = append(streamerObjs, s)
		m.initialPoints[s.Username] = s.ChannelPoints
	}

	if len(streamerObjs) > 0 {
		m.logger.EmojiPrintf(":white_check_mark:", "%d Streamer loaded! (%s)", len(streamerObjs), formatLoadDuration(time.Since(loadStartedAt)))
	}

	if m.ClaimDropsStartup && m.StreamerSettings.ClaimDrops {
		if drops, err := m.twitch.ClaimAllDropsFromInventory(); err != nil {
			m.logger.Printf("startup drop claim failed: %v", err)
		} else {
			m.logClaimedDrops(drops)
		}
	}

	m.streamers = streamerObjs

	// ? background loops
	if m.StreamerSettings.ClaimDrops {
		go m.dropClaimer(m.stop)
	}
	go m.contextRefresher(streamerObjs, m.stop)
	go m.minuteWatcher(streamerObjs, m.stop)
	go m.startPubSub(streamerObjs, m.stop)

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
			if drops, err := m.twitch.ClaimAllDropsFromInventory(); err != nil {
				m.logger.Printf("drop claim failed: %v", err)
			} else {
				m.logClaimedDrops(drops)
			}
		case <-stop:
			return
		}
	}
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

func (m *Miner) contextRefresher(streamers []*entities.Streamer, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, s := range streamers {
				prev := s.ChannelPoints
				if _, err := m.twitch.LoadChannelPointsContext(s); err != nil {
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
				if _, err := m.twitch.CheckStreamerOnline(streamer); err != nil {
					m.logger.Printf("online check %s: %v", m.styledStreamerName(streamer), err)
				}
				if !streamer.IsOnline {
					continue
				}
			}

			if err := m.twitch.SendMinuteWatched(streamer); err != nil {
				if errors.Is(err, classpkg.ErrStreamerOffline) {
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
				m.logger.Errorf("minute watch %s: %v", m.styledStreamerName(streamer), err)
			}

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
	if err := m.twitch.UpdateStream(streamer); err != nil {
		if errors.Is(err, classpkg.ErrStreamerOffline) {
			m.setPresence(streamer, false, "stream-info")
		} else {
			m.logger.Debugf("stream info %s: %v", m.styledStreamerName(streamer), err)
		}
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
	if err := m.twitch.UpdateStream(streamer); err != nil {
		if m.logger != nil && m.logger.DebugEnabled() {
			m.logger.Debugf("update stream %s for game: %v", m.styledStreamerName(streamer), err)
		}
		return ""
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
		if !s.OnlineAt.IsZero() && now.Sub(s.OnlineAt) < 30*time.Second {
			continue
		}
		m.refreshStreamForPreference(s)
		if s == nil || !s.IsOnline {
			continue
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
	add := func(c candidate, reason string) {
		if len(selected) >= maxConcurrentWatchers {
			return
		}
		if _, ok := seen[c.idx]; ok {
			return
		}
		game := c.game
		if game != "" {
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

	pick := func(list []candidate, includeGameRank bool, less func(a, b candidate) bool, reason string) {
		for _, c := range sortCandidates(list, less, includeGameRank) {
			add(c, reason)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}

	skipEarlyStreak := len(m.gamePriority) > 0 && !hasPriorityGameStreak

	for _, priority := range m.watchPriorities {
		if len(selected) >= maxConcurrentWatchers {
			break
		}
		switch priority {
		case watchPriorityOrder:
			pick(candidates, false, nil, "ORDER")
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
			pick(streaks, true, nil, "STREAK")
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
			pick(drops, true, nil, "DROPS")
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
			}, "SUBSCRIBED")
		case watchPriorityPointsAscending:
			asc := append([]candidate(nil), candidates...)
			pick(asc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints < streamers[b.idx].ChannelPoints
			}, "POINTS_ASC")
		case watchPriorityPointsDescending:
			desc := append([]candidate(nil), candidates...)
			pick(desc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints > streamers[b.idx].ChannelPoints
			}, "POINTS_DESC")
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
				add(*streakPick, "FORCE_STREAK_SLOT2")
			} else {
				keepIdx := selected[0]
				selected = selected[:0]
				seen = make(map[int]struct{})
				selectedGames = make(map[string]struct{})
				if keepCand, ok := candidateByIdx[keepIdx]; ok {
					add(keepCand, selectedReason[keepIdx])
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
		pick(candidates, true, nil, "FALLBACK")
	}

	watchList := make([]*entities.Streamer, 0, len(selected))
	for _, idx := range selected {
		watchList = append(watchList, streamers[idx])
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

func (m *Miner) shouldPrioritizeStreak(streamer *entities.Streamer, now time.Time) bool {
	if streamer == nil || streamer.Stream == nil {
		return false
	}
	if !streamer.Settings.WatchStreak || !streamer.Stream.WatchStreakMissing {
		return false
	}
	if !streamer.OfflineAt.IsZero() && now.Sub(streamer.OfflineAt) <= 30*time.Minute {
		return false
	}
	// ? Keep streak priority long enough for Twitch to issue the streak check (typically ~15 minutes).
	return streamer.Stream.MinuteWatched < m.streakPriorityLimit(now)
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
		totalPointsChange += s.ChannelPoints - m.initialPoints[s.Username]
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
		initial := m.initialPoints[s.Username]
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
	online, err := m.twitch.CheckStreamerOnline(streamer)
	if err != nil {
		m.logger.Printf("online check %s: %v", m.styledStreamerName(streamer), err)
		return
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
	m.logger.EmojiEventf(":partying_face:", constants.EventStreamerOnline, "%s (%s%s%s points) is %sOnline%s!%s", name, colorCyan, points, colorReset, colorGreen, colorReset, gameSuffix)
}

func (m *Miner) logOffline(streamer *entities.Streamer) {
	name := m.styledStreamerName(streamer)
	points := m.formattedStreamerPoints(streamer)
	m.logger.EmojiEventf(":sleeping:", constants.EventStreamerOffline, "%s (%s%s%s points) is %sOffline%s!", name, colorCyan, points, colorReset, colorRed, colorReset)
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
}

func (m *Miner) updateHistory(streamer *entities.Streamer, reason string, amount int) {
	if reason == "" {
		return
	}
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
	if reason == "WATCH_STREAK" && streamer.Stream != nil {
		streamer.Stream.WatchStreakMissing = false
	}
}

func (m *Miner) handlePubSubPresence(streamer *entities.Streamer, online bool, reason string) {
	m.setPresence(streamer, online, fmt.Sprintf("pubsub:%s", reason))
}

func (m *Miner) setPresence(streamer *entities.Streamer, online bool, reason string) {
	prevKnown := streamer.PresenceKnown
	prevOnline := streamer.IsOnline
	streamer.PresenceKnown = true
	if online != prevOnline || !prevKnown {
		if online {
			streamer.OnlineAt = time.Now()
		} else {
			streamer.OfflineAt = time.Now()
		}
	}
	streamer.IsOnline = online
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
		return
	}
	if prevOnline != online {
		if online {
			m.logOnline(streamer)
		} else {
			m.logOffline(streamer)
		}
		return
	}
	if reason != "" && !online {
		// ? Offline message already logged for state changes; keep silent on no-op toggles.
		return
	}
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
	watcher := classpkg.NewChatClient(m.Username, token, streamer.Username, m.logger, false, m.anonymizer)
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
