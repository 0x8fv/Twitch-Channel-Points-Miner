package miner

import (
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/streak"
)

// pickStreamersToWatch chooses which online streamers get the watch slots this round.
func (m *Miner) pickStreamersToWatch(streamers []*domain.Streamer) []*domain.Streamer {
	now := time.Now()
	candidates := m.watchCandidates(streamers, now)
	streakReady := streakReadyCandidates(candidates)
	slots := newWatchSlots(candidates)

	m.keepActiveStreaks(slots, streamers, now)

	// With a game priority list, a streak on a game outside it does not jump the queue.
	skipEarlyStreak := len(m.gamePriority) > 0 && !anyPriorityGame(streakReady)
	for _, priority := range m.watchPriorities {
		if slots.full() {
			break
		}
		if priority == watchPriorityStreak && skipEarlyStreak {
			continue
		}
		m.pickByPriority(slots, priority, streamers, now)
	}

	slots.ensureStreak(streakReady)
	if skipEarlyStreak {
		slots.preferPriorityGameFirst()
	}
	if !slots.full() {
		slots.pick(candidates, true, nil, "FALLBACK", false)
	}

	watchList := make([]*domain.Streamer, 0, len(slots.picked))
	for _, idx := range slots.picked {
		watchList = append(watchList, streamers[idx])
		m.syncActiveStreakWatch(streamers[idx], now)
	}
	m.logWatchQueueSlots(slots, streamers, now)
	return watchList
}

// keepActiveStreaks gives a slot to every streamer whose watch streak is already in
// progress, and forgets the ones that no longer qualify.
func (m *Miner) keepActiveStreaks(slots *watchSlots, streamers []*domain.Streamer, now time.Time) {
	m.activeStreakMu.Lock()
	defer m.activeStreakMu.Unlock()
	if len(m.activeStreakWatches) == 0 {
		return
	}
	activeStreaks := make([]watchCandidate, 0, len(m.activeStreakWatches))
	for _, c := range slots.candidates {
		s := streamers[c.idx]
		if s == nil {
			continue
		}
		key := m.activeStreakWatchKey(s)
		watch, ok := m.activeStreakWatches[key]
		if !ok {
			continue
		}
		if !m.shouldKeepActiveStreak(s, now) || !streak.ActiveWatchMatches(s, watch) {
			delete(m.activeStreakWatches, key)
			continue
		}
		activeStreaks = append(activeStreaks, c)
	}
	slots.pick(activeStreaks, false, nil, "ACTIVE_STREAK", true)
}

// pickByPriority fills free slots with the candidates one watch priority asks for.
func (m *Miner) pickByPriority(slots *watchSlots, priority watchPriority, streamers []*domain.Streamer, now time.Time) {
	candidates := slots.candidates
	switch priority {
	case watchPriorityOrder:
		slots.pick(candidates, false, nil, "ORDER", false)
	case watchPriorityStreak:
		streaks := make([]watchCandidate, 0, len(candidates))
		for _, c := range candidates {
			if m.shouldPrioritizeStreak(streamers[c.idx], now) {
				streaks = append(streaks, c)
			}
		}
		slots.pick(streaks, true, nil, "STREAK", true)
	case watchPriorityDrops:
		drops := make([]watchCandidate, 0, len(candidates))
		for _, c := range candidates {
			s := streamers[c.idx]
			if s == nil || s.Stream == nil {
				continue
			}
			if s.Settings.ClaimDrops {
				drops = append(drops, c)
			}
		}
		slots.pick(drops, true, nil, "DROPS", false)
	case watchPrioritySubscribed:
		subscribed := make([]watchCandidate, 0, len(candidates))
		for _, c := range candidates {
			s := streamers[c.idx]
			if s == nil {
				continue
			}
			if s.HasActiveMultipliers() {
				subscribed = append(subscribed, c)
			}
		}
		slots.pick(subscribed, true, func(a, b watchCandidate) bool {
			return streamers[a.idx].TotalMultiplier() > streamers[b.idx].TotalMultiplier()
		}, "SUBSCRIBED", false)
	case watchPriorityPointsAscending:
		slots.pick(candidates, true, func(a, b watchCandidate) bool {
			return streamers[a.idx].ChannelPoints < streamers[b.idx].ChannelPoints
		}, "POINTS_ASC", false)
	case watchPriorityPointsDescending:
		slots.pick(candidates, true, func(a, b watchCandidate) bool {
			return streamers[a.idx].ChannelPoints > streamers[b.idx].ChannelPoints
		}, "POINTS_DESC", false)
	}
}

//	func (m *Miner) logAvailableCampaigns() {
//		if m.logger == nil || m.twitch == nil {
//			return
//		}
//		summaries := m.twitch.AvailableCampaignSummaries()
//		if len(summaries) == 0 {
//			m.logger.Printf("Active drop campaigns: none detected")
//			return
//		}
//		m.logger.Printf("Active drop campaigns (%d):", len(summaries))
//		for _, summary := range summaries {
//			m.logger.Printf(" - %s", summary)
//		}
//	}
func (m *Miner) gamePreference(streamer *domain.Streamer) (int, bool) {
	baseRank := len(m.gamePriority) + 1
	if streamer == nil || streamer.Stream == nil {
		return baseRank, false
	}
	name := strings.ToLower(strings.TrimSpace(streamer.Stream.GameName()))
	if name == "" {
		return baseRank, false
	}
	if _, ok := m.gameExclusions[name]; ok {
		return 0, true
	}
	if idx, ok := m.gamePriorityIndex[name]; ok {
		return idx, false
	}
	return baseRank, false
}
