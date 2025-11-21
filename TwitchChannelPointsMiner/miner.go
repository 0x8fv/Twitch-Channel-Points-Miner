package twitchchannelpointsminer

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	classpkg "TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/utils"
)

const (
	colorGreen = "\033[38;5;46m"
	colorRed   = "\033[38;5;196m"
	colorCyan  = "\033[38;5;14m"
	colorReset = "\033[0m"
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

type Miner struct {
	Username                   string
	Password                   string
	ClaimDropsStartup          bool
	DisableSSLCertVerification bool
	LoggerSettings             LoggerSettings
	StreamerSettings           entities.StreamerSettings
	logger                     *Logger
	startedAt                  time.Time
	twitch                     *classpkg.Twitch
	streamers                  []*entities.Streamer
	initialPoints              map[string]int
	stop                       chan struct{}
	watchPriorities            []watchPriority
}

func NewMiner(username, password string, claimDropsStartup bool, disableCertCheck bool, loggerSettings LoggerSettings, streamerSettings entities.StreamerSettings, priorityNames []string) *Miner {
	streamerSettings.Default()
	return &Miner{
		Username:                   username,
		Password:                   password,
		ClaimDropsStartup:          claimDropsStartup,
		DisableSSLCertVerification: disableCertCheck,
		LoggerSettings:             loggerSettings,
		StreamerSettings:           streamerSettings,
		logger:                     NewLogger(loggerSettings, username),
		watchPriorities:            parseWatchPriorities(priorityNames),
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

func (m *Miner) run(streamers []string, useFollowers bool, order entities.FollowersOrder) {
	m.startedAt = time.Now()
	m.logger.Printf("Twitch Channel Points Miner | v%s", constants.Version)
	m.logger.Println("https://github.com/0x8fv/Twitch-Channel-Points-Miner")
	sessionID := newSessionID()
	m.logger.EmojiPrintf(":green_circle:", "Start session: '%s'", sessionID)
	m.stop = make(chan struct{})
	m.initialPoints = make(map[string]int)

	tw, err := classpkg.NewTwitch(m.Username, utils.GetUserAgent("CHROME"), m.Password)
	if err != nil {
		m.logger.Fatalf("failed to create twitch client: %v", err)
	}
	m.twitch = tw
	if err := m.twitch.Login(m.Username); err != nil {
		m.logger.Fatalf("login failed: %v", err)
	}

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

	streamerObjs := make([]*entities.Streamer, 0, len(targets))
	m.logger.EmojiPrintf(":hourglass_flowing_sand:", "Loading data for %d streamer(s). Please wait...", len(targets))
	for _, name := range targets {
		if name == "" {
			continue
		}
		s := &entities.Streamer{
			Username:    name,
			Settings:    m.StreamerSettings,
			Stream:      entities.NewStream(),
			StreamerURL: fmt.Sprintf("%s/%s", constants.URL, name),
		}
		id, err := m.twitch.GetChannelID(name)
		if err != nil {
			m.logger.Printf("skip %s: %v", name, err)
			continue
		}
		s.ChannelID = id
		prev := s.ChannelPoints
		if _, err := m.twitch.LoadChannelPointsContext(s); err != nil {
			m.logger.Printf("context for %s: %v", name, err)
		} else {
			m.handlePointsUpdate(s, prev, "")
		}
		m.updatePresence(s)
		streamerObjs = append(streamerObjs, s)
		m.initialPoints[s.Username] = s.ChannelPoints
	}

	if len(streamerObjs) > 0 {
		m.logger.EmojiPrintf(":white_check_mark:", "%d Streamer loaded!", len(streamerObjs))
	}

	if m.ClaimDropsStartup {
		if drops, err := m.twitch.ClaimAllDropsFromInventory(); err != nil {
			m.logger.Printf("startup drop claim failed: %v", err)
		} else {
			m.logClaimedDrops(drops)
		}
	}

	m.streamers = streamerObjs

	// ? background loops
	go m.dropClaimer(m.stop)
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
		m.logger.EmojiPrintf(":package:", "Claim %s (%s) %s (%d%%)", reward, campaign, progress, percent)
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
					m.logger.Printf("refresh %s: %v", s.Username, err)
				} else {
					m.handlePointsUpdate(s, prev, "")
					if s.Settings.ClaimDrops && s.Stream != nil {
						if campaigns, err := m.twitch.CampaignIDsForStreamer(s); err == nil {
							s.Stream.CampaignIDs = campaigns
						}
					}
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
					m.logger.Printf("online check %s: %v", streamer.Username, err)
				}
				if !streamer.IsOnline {
					continue
				}
			}

			if err := m.twitch.SendMinuteWatched(streamer); err != nil {
				m.logger.Printf("minute watch %s: %v", streamer.Username, err)
			}

			if m.sleepWithStop(interval, stop) {
				return
			}
		}
	}
}

func (m *Miner) pickStreamersToWatch(streamers []*entities.Streamer) []*entities.Streamer {
	now := time.Now()
	candidates := make([]int, 0, len(streamers))
	for idx, s := range streamers {
		if s == nil || !s.IsOnline {
			continue
		}
		if !s.OnlineAt.IsZero() && now.Sub(s.OnlineAt) < 30*time.Second {
			continue
		}
		candidates = append(candidates, idx)
	}

	selected := make([]int, 0, maxConcurrentWatchers)
	seen := make(map[int]struct{})
	add := func(idx int) {
		if len(selected) >= maxConcurrentWatchers {
			return
		}
		if _, ok := seen[idx]; ok {
			return
		}
		seen[idx] = struct{}{}
		selected = append(selected, idx)
	}

	pick := func(indices []int) {
		for _, idx := range indices {
			add(idx)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}

	for _, priority := range m.watchPriorities {
		if len(selected) >= maxConcurrentWatchers {
			break
		}
		switch priority {
		case watchPriorityOrder:
			pick(candidates)
		case watchPriorityStreak:
			streaks := make([]int, 0, len(candidates))
			for _, idx := range candidates {
				if m.shouldPrioritizeStreak(streamers[idx], now) {
					streaks = append(streaks, idx)
				}
			}
			pick(streaks)
		case watchPriorityDrops:
			drops := make([]int, 0, len(candidates))
			for _, idx := range candidates {
				s := streamers[idx]
				if s == nil || s.Stream == nil {
					continue
				}
				if s.Settings.ClaimDrops && len(s.Stream.CampaignIDs) > 0 {
					drops = append(drops, idx)
				}
			}
			pick(drops)
		case watchPrioritySubscribed:
			subscribed := append([]int(nil), candidates...)
			sort.SliceStable(subscribed, func(i, j int) bool {
				return streamers[subscribed[i]].TotalMultiplier() > streamers[subscribed[j]].TotalMultiplier()
			})
			pick(subscribed)
		case watchPriorityPointsAscending:
			asc := append([]int(nil), candidates...)
			sort.SliceStable(asc, func(i, j int) bool {
				return streamers[asc[i]].ChannelPoints < streamers[asc[j]].ChannelPoints
			})
			pick(asc)
		case watchPriorityPointsDescending:
			desc := append([]int(nil), candidates...)
			sort.SliceStable(desc, func(i, j int) bool {
				return streamers[desc[i]].ChannelPoints > streamers[desc[j]].ChannelPoints
			})
			pick(desc)
		}
	}

	if len(selected) < maxConcurrentWatchers {
		pick(candidates)
	}

	watchList := make([]*entities.Streamer, 0, len(selected))
	for _, idx := range selected {
		watchList = append(watchList, streamers[idx])
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
	return streamer.Stream.MinuteWatched < 7
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
		streamers,
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
	m.logger.EmojiPrintf(":stop_sign:", "Ending session: '%s'", sessionID)
	duration := formatDuration(time.Since(m.startedAt))
	m.logger.EmojiPrintf(":hourglass:", "Duration %s", duration)
	for _, s := range m.streamers {
		initial := m.initialPoints[s.Username]
		total := s.ChannelPoints - initial
		if total == 0 && (s.History == nil || len(s.History) == 0) {
			continue
		}
		signColor := colorGreen
		sign := "+"
		if total < 0 {
			signColor = colorRed
			sign = "-"
			total = -total
		}
		points := formatChannelPoints(s.ChannelPoints)
		m.logger.EmojiPrintf(":moneybag:", "%s (%s%s%s points), Total Points %s%s%d%s", displayName(s.Username), colorCyan, points, colorReset, signColor, sign, total, colorReset)
		if s.History != nil {
			for reason, entry := range s.History {
				m.logger.Printf("                         %s (%d times, %d gained)", reason, entry.Count, entry.Amount)
			}
		}
	}
	os.Exit(0)
}

func (m *Miner) updatePresence(streamer *entities.Streamer) {
	online, err := m.twitch.CheckStreamerOnline(streamer)
	if err != nil {
		m.logger.Printf("online check %s: %v", streamer.Username, err)
		return
	}
	m.setPresence(streamer, online, "poll")
}

func (m *Miner) logOnline(streamer *entities.Streamer) {
	name := displayName(streamer.Username)
	m.logger.EmojiPrintf(":speech_balloon:", "Join IRC Chat: %s", streamer.Username)
	points := formatChannelPoints(streamer.ChannelPoints)
	m.logger.EmojiPrintf(":partying_face:", "%s (%s%s%s points) is %sOnline%s!", name, colorCyan, points, colorReset, colorGreen, colorReset)
}

func (m *Miner) logOffline(streamer *entities.Streamer) {
	name := displayName(streamer.Username)
	points := formatChannelPoints(streamer.ChannelPoints)
	m.logger.EmojiPrintf(":sleeping:", "%s (%s%s%s points) is %sOffline%s!", name, colorCyan, points, colorReset, colorRed, colorReset)
}

func displayName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
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
	name := displayName(streamer.Username)
	points := formatChannelPoints(streamer.ChannelPoints)
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
		reason,
	)
}

func (m *Miner) handlePubSubGain(streamer *entities.Streamer, earned int, reason string, balance int) {
	prev := streamer.ChannelPoints
	streamer.ChannelPoints = balance
	if !streamer.PointsInit {
		streamer.PointsInit = true
	}
	delta := streamer.ChannelPoints - prev
	if delta == 0 {
		delta = earned
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
