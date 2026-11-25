package miner

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

// maxNamesInSummary keeps the load summary to one readable line.
const maxNamesInSummary = 10

// loadStreamers looks up every target a few at a time and returns the ones that
// exist, in the order they were asked for. Nobody's presence is announced while
// this runs; logLoadSummary reports everyone at once.
func (m *Miner) loadStreamers(targets []string) []*domain.Streamer {
	m.startingUp.Store(true)
	defer m.startingUp.Store(false)

	loaded := make([]*domain.Streamer, len(targets))
	forEachIndex(len(targets), streamerConcurrency, func(i int) {
		loaded[i] = m.loadStreamer(targets[i])
	})

	streamers := make([]*domain.Streamer, 0, len(loaded))
	m.initialPointsMu.Lock()
	for _, s := range loaded {
		if s == nil {
			continue
		}
		streamers = append(streamers, s)
		m.initialPoints[streamerInitialPointsKey(s)] = s.ChannelPoints
	}
	m.initialPointsMu.Unlock()
	return streamers
}

// loadStreamer builds one streamer with its channel id, points and presence.
// It returns nil when the channel cannot be found.
func (m *Miner) loadStreamer(name string) *domain.Streamer {
	if name == "" {
		return nil
	}
	settings := m.StreamerSettings
	if override, ok := m.StreamerOverrides[strings.ToLower(name)]; ok {
		settings = override
	}
	s := &domain.Streamer{
		Username:    name,
		Settings:    settings,
		Stream:      domain.NewStream(),
		StreamerURL: fmt.Sprintf("%s/%s", twitchapi.URL, name),
	}
	id, err := m.twitch.GetChannelID(name)
	if err != nil {
		// if channel id can't be gotten from twitch, try checking warm start cache
		// in case streamer has changed names recently
		id = ""
		if m.warmStartCache != nil {
			entry, ok := m.warmStartCache.Get(name)
			if ok {
				id = strings.TrimSpace(entry.ChannelID)
			}
		}
		if id == "" {
			m.logger.Printf("skip %s: %v", m.rawStreamerName(name), err)
			return nil
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
	return s
}

// logLoadSummary reports the loaded streamers in one line instead of one per streamer.
func (m *Miner) logLoadSummary(streamers []*domain.Streamer, took time.Duration) {
	if len(streamers) == 0 {
		return
	}
	m.logger.EmojiEventf(":white_check_mark:", events.EventStartup, "%s", m.loadSummary(streamers, took))
}

func (m *Miner) loadSummary(streamers []*domain.Streamer, took time.Duration) string {
	var online []string
	for _, s := range streamers {
		if s.IsOnline {
			online = append(online, m.styledStreamerName(s))
		}
	}
	noun := "streamers"
	if len(streamers) == 1 {
		noun = "streamer"
	}
	summary := fmt.Sprintf("%d %s loaded in %s: %d online", len(streamers), noun, formatLoadDuration(took), len(online))
	if len(online) > 0 {
		shown := online
		if len(shown) > maxNamesInSummary {
			shown = shown[:maxNamesInSummary]
		}
		names := strings.Join(shown, ", ")
		if hidden := len(online) - len(shown); hidden > 0 {
			names += fmt.Sprintf(" and %d more", hidden)
		}
		summary += " (" + names + ")"
	}
	return summary + fmt.Sprintf(", %d offline", len(streamers)-len(online))
}
