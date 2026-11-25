package miner

import (
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitch"
)

func (m *Miner) updateChatPresence(streamer *domain.Streamer, online bool) {
	if streamer == nil {
		return
	}
	if shouldJoinChat(streamer.Settings.IRCMode, online) {
		m.startChatWatcher(streamer)
		return
	}
	m.stopChatWatcher(streamer)
}

func (m *Miner) startChatWatcher(streamer *domain.Streamer) {
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
	watcher := twitch.NewChatClient(m.Username, token, streamer.Username, m.logger, m.disableAtInNickname, m.anonymizer)
	m.chatWatchers[key] = watcher
	m.chatMu.Unlock()
	if m.logger != nil {
		m.logger.EmojiPrintf(":speech_balloon:", "Join IRC Chat: %s", m.styledStreamerName(streamer))
	}
	watcher.Start()
}

func (m *Miner) stopChatWatcher(streamer *domain.Streamer) {
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
	watchers := make([]*twitch.ChatClient, 0, len(m.chatWatchers))
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

func shouldJoinChat(mode domain.IRCMode, online bool) bool {
	switch mode {
	case domain.IRCModeAlways:
		return true
	case domain.IRCModeNever:
		return false
	case domain.IRCModeOffline:
		return !online
	case domain.IRCModeOnline:
		return online
	default:
		return online
	}
}
