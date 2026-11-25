package miner

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

// logWatchQueueSlots prints who holds each watch slot and why, when queue logging is on.
func (m *Miner) logWatchQueueSlots(slots *watchSlots, streamers []*domain.Streamer, now time.Time) {
	if m.logger == nil || !m.logWatchQueue {
		return
	}
	interval := m.watchInterval(len(slots.picked))
	lines := make([]string, 0, len(slots.picked)+1)
	lines = append(lines, fmt.Sprintf("WATCH queue (≈%s between streamers):", formatDuration(interval)))
	for slot, idx := range slots.picked {
		s := streamers[idx]
		cand := slots.byIdx[idx]
		reason := slots.reasons[idx]
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
