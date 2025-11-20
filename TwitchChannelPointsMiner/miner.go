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
		if err := m.twitch.ClaimAllDropsFromInventory(); err != nil {
			m.logger.Printf("startup drop claim failed: %v", err)
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
			if err := m.twitch.ClaimAllDropsFromInventory(); err != nil {
				m.logger.Printf("drop claim failed: %v", err)
			}
		case <-stop:
			return
		}
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
				m.updatePresence(s)
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
	client := classpkg.NewPubSubClient(m.twitch, m.logger, streamers, m.handlePubSubGain)
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
	m.logger.EmojiPrintf(":hourglass:", "Duration %s", time.Since(m.startedAt))
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
		m.logger.EmojiPrintf(":moneybag:", "%s (%s%.2fk%s points), Total Points %s%s%d%s", displayName(s.Username), colorCyan, float64(s.ChannelPoints)/1000, colorReset, signColor, sign, total, colorReset)
		if s.History != nil {
			for reason, entry := range s.History {
				m.logger.Printf("                         %s (%d times, %d gained)", reason, entry.Count, entry.Amount)
			}
		}
	}
	os.Exit(0)
}

func (m *Miner) updatePresence(streamer *entities.Streamer) {
	prev := streamer.IsOnline
	online, err := m.twitch.CheckStreamerOnline(streamer)
	if err != nil {
		m.logger.Printf("online check %s: %v", streamer.Username, err)
		return
	}
	if !streamer.PresenceKnown {
		if online {
			m.logOnline(streamer)
		} else {
			m.logOffline(streamer)
		}
		streamer.PresenceKnown = true
		return
	}
	if online != prev {
		if online {
			m.logOnline(streamer)
		} else {
			m.logOffline(streamer)
		}
	}
}

func (m *Miner) logOnline(streamer *entities.Streamer) {
	name := displayName(streamer.Username)
	m.logger.EmojiPrintf(":speech_balloon:", "Join IRC Chat: %s", streamer.Username)
	m.logger.EmojiPrintf(":partying_face:", "%s (%s%d%s points) is %sOnline%s!", name, colorCyan, streamer.ChannelPoints, colorReset, colorGreen, colorReset)
}

func (m *Miner) logOffline(streamer *entities.Streamer) {
	name := displayName(streamer.Username)
	m.logger.EmojiPrintf(":sleeping:", "%s (%s%d%s points) is %sOffline%s!", name, colorCyan, streamer.ChannelPoints, colorReset, colorRed, colorReset)
}

func displayName(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
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
		"%s%s%d%s → %s (%s%.2fk%s points) - Reason: %s",
		valueColor,
		sign,
		delta,
		colorReset,
		name,
		colorCyan,
		float64(streamer.ChannelPoints)/1000,
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
