package miner

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/twitch"
	"TwitchChannelPointsMiner/internal/update"
)

func (m *Miner) run(streamers []string, useFollowers bool, order domain.FollowersOrder) {
	m.startedAt = time.Now()
	m.logger.Printf("Twitch Channel Points Miner | v%s", update.Version)
	m.logger.Println("https://github.com/0x8fv/Twitch-Channel-Points-Miner")
	sessionID := newSessionID()
	m.logger.EmojiEventf(":green_circle:", events.EventStartup, "Start session: '%s'", sessionID)
	m.stop = make(chan struct{})
	m.initialPoints = make(map[string]int)

	tw, err := twitch.NewTwitch(m.Username, twitch.GetUserAgent("CHROME"), m.Password, m.logger, m.anonymizer)
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

	streamerObjs := make([]*domain.Streamer, 0, len(targets))
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
		s := &domain.Streamer{
			Username:    name,
			Settings:    settings,
			Stream:      domain.NewStream(),
			StreamerURL: fmt.Sprintf("%s/%s", twitch.URL, name),
		}
		id, err := m.twitch.GetChannelID(name)
		if err != nil {
			// if channel id can't be gotten from twitch, try checking warm start cache
			// in case streamer has changed names recently
			id = ""
			if m.warmStartCache != nil {
				entry, ok := m.warmStartCache.get(name)
				if ok {
					id = strings.TrimSpace(entry.ChannelID)
				}
			}
			if id == "" {
				m.logger.Printf("skip %s: %v", m.rawStreamerName(name), err)
				continue
			}
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
	go m.analyticsSnapshotter(m.stop)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	m.shutdown(sessionID)
}

func (m *Miner) contextRefresher(streamers []*domain.Streamer, stop <-chan struct{}) {
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

func (m *Miner) startPubSub(streamers []*domain.Streamer, stop <-chan struct{}) {
	client := twitch.NewPubSubClient(
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
	m.logger.EmojiEventf(":stop_sign:", events.EventShutdown, "Ending session: '%s'", sessionID)
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
