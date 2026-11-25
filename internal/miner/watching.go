package miner

import (
	"errors"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitch"
)

const (
	minuteWatchFailureBackoffThreshold = 3
	minuteWatchFailureBackoffBase      = 5 * time.Minute
	minuteWatchFailureBackoffMax       = 30 * time.Minute
)

func streamerWatchBackedOff(streamer *domain.Streamer, now time.Time) bool {
	if streamer == nil || streamer.WatchBackoffUntil.IsZero() {
		return false
	}
	return now.Before(streamer.WatchBackoffUntil)
}

func recordMinuteWatchFailure(streamer *domain.Streamer, now time.Time) {
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

func recordMinuteWatchSuccess(streamer *domain.Streamer) {
	if streamer == nil {
		return
	}
	streamer.WatchFailureCount = 0
	streamer.WatchBackoffUntil = time.Time{}
}

func (m *Miner) minuteWatcher(streamers []*domain.Streamer, stop <-chan struct{}) {
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
				if errors.Is(err, twitch.ErrStreamerOffline) {
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

func (m *Miner) refreshStreamForPreference(streamer *domain.Streamer) {
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
		if errors.Is(err, twitch.ErrStreamerOffline) {
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
