package miner

import (
	"fmt"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/streak"
)

func (m *Miner) handlePointsUpdate(streamer *domain.Streamer, previous int, reason string) {
	if !streamer.PointsInit {
		streamer.PointsInit = true
		m.analyticsRecordPoint(streamer, 0, "")
		return
	}
	delta := streamer.ChannelPoints - previous
	m.logPointsDelta(streamer, delta, reason)
}

func (m *Miner) logPointsDelta(streamer *domain.Streamer, delta int, reason string) {
	if delta == 0 {
		return
	}
	m.analyticsRecordPoint(streamer, delta, reason)
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
	event := events.EventFromGainReason(reason)
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
		mark := "➕"
		if sign == "-" {
			mark = "➖"
		}
		m.logger.EmojiPrintf(
			mark,
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

func (m *Miner) handlePubSubGain(streamer *domain.Streamer, earned int, reason string, balance int) {
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

func (m *Miner) updateHistory(streamer *domain.Streamer, reason string, amount int) {
	if reason == "" {
		return
	}
	streamer.HistoryMu.Lock()
	if streamer.History == nil {
		streamer.History = make(map[string]*domain.HistoryEntry)
	}
	entry, ok := streamer.History[reason]
	if !ok {
		entry = &domain.HistoryEntry{}
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
		if streamer.Stream.WatchStreakMissing && streamer.Stream.WatchCount >= 2 {
			streak.MarkCompleted(streamer)
		}
		m.syncActiveStreakWatch(streamer, time.Now())
		return
	}
	if reason == "WATCH_STREAK" {
		streak.MarkCompleted(streamer)
	}
	m.syncActiveStreakWatch(streamer, time.Now())
}
