package miner

import (
	"errors"
	"fmt"
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitch"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func streamerInitialPointsKey(streamer *domain.Streamer) string {
	if streamer == nil {
		return ""
	}
	if id := strings.TrimSpace(streamer.ChannelID); id != "" {
		return id
	}
	return strings.ToLower(strings.TrimSpace(streamer.Username))
}

func (m *Miner) applyStreamerLoginChange(streamer *domain.Streamer, login string) bool {
	if streamer == nil {
		return false
	}
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || strings.EqualFold(streamer.Username, login) {
		return false
	}

	oldLogin := strings.ToLower(strings.TrimSpace(streamer.Username))
	if m != nil {
		m.chat.Leave(streamer)
	}

	streamer.Username = login
	streamer.StreamerURL = fmt.Sprintf("%s/%s", twitchapi.URL, login)
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
			m.initialPointsMu.Lock()
			if initial, ok := m.initialPoints[oldLogin]; ok {
				m.initialPoints[stableKey] = initial
			}
			m.initialPointsMu.Unlock()
		}
	}
	if m != nil {
		m.chat.Update(streamer, streamer.IsOnline)
		m.syncWarmStartCacheFromStreamer(streamer)
	}
	return true
}

func shouldRepairStreamerIdentity(err error) bool {
	return errors.Is(err, twitch.ErrChannelNotFound)
}

func (m *Miner) repairStreamerIdentity(streamer *domain.Streamer, err error) bool {
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

func (m *Miner) loadChannelPointsContextWithIdentityRepair(streamer *domain.Streamer) (int, error) {
	points, err := m.twitch.LoadChannelPointsContext(streamer)
	if err == nil {
		return points, nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return points, err
	}
	return m.twitch.LoadChannelPointsContext(streamer)
}

func (m *Miner) checkStreamerOnlineWithIdentityRepair(streamer *domain.Streamer) (bool, error) {
	online, err := m.twitch.CheckStreamerOnline(streamer)
	if err == nil {
		return online, nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return online, err
	}
	return m.twitch.CheckStreamerOnline(streamer)
}

func (m *Miner) updateStreamWithIdentityRepair(streamer *domain.Streamer) error {
	err := m.twitch.UpdateStream(streamer)
	if err == nil {
		return nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return err
	}
	return m.twitch.UpdateStream(streamer)
}

func (m *Miner) sendMinuteWatchedWithIdentityRepair(streamer *domain.Streamer) error {
	err := m.twitch.SendMinuteWatched(streamer)
	if err == nil {
		return nil
	}
	if !m.repairStreamerIdentity(streamer, err) {
		return err
	}
	return m.twitch.SendMinuteWatched(streamer)
}
