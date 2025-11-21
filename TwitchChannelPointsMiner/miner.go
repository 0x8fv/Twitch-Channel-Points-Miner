package twitchchannelpointsminer

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
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
}

func NewMiner(username, password string, claimDropsStartup bool, disableCertCheck bool, loggerSettings LoggerSettings, streamerSettings entities.StreamerSettings) *Miner {
	streamerSettings.Default()
	return &Miner{
		Username:                   username,
		Password:                   password,
		ClaimDropsStartup:          claimDropsStartup,
		DisableSSLCertVerification: disableCertCheck,
		LoggerSettings:             loggerSettings,
		StreamerSettings:           streamerSettings,
		logger:                     NewLogger(loggerSettings, username),
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
				}
			}
		case <-stop:
			return
		}
	}
}

func (m *Miner) minuteWatcher(streamers []*entities.Streamer, stop <-chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			watched := 0
			for _, s := range streamers {
				if watched >= 2 {
					break
				}
				if !s.IsOnline {
					continue
				}
				if err := m.twitch.SendMinuteWatched(s); err != nil {
					m.logger.Printf("minute watch %s: %v", s.Username, err)
					continue
				}
				watched++
			}
		case <-stop:
			return
		}
	}
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
}

func (m *Miner) handlePubSubPresence(streamer *entities.Streamer, online bool, reason string) {
	m.setPresence(streamer, online, fmt.Sprintf("pubsub:%s", reason))
}

func (m *Miner) setPresence(streamer *entities.Streamer, online bool, reason string) {
	prevKnown := streamer.PresenceKnown
	prevOnline := streamer.IsOnline
	streamer.PresenceKnown = true
	streamer.IsOnline = online
	if online {
		streamer.OnlineAt = time.Now()
	} else {
		streamer.OfflineAt = time.Now()
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
