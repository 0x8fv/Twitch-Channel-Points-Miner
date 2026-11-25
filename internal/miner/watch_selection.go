package miner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

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

func (m *Miner) pickStreamersToWatch(streamers []*domain.Streamer) []*domain.Streamer {
	now := time.Now()
	type candidate struct {
		idx           int
		rank          int
		game          string
		position      int
		priorityGame  bool
		isStreakReady bool
	}
	candidates := make([]candidate, 0, len(streamers))
	candidateByIdx := make(map[int]candidate, len(streamers))
	streakCandidates := make([]candidate, 0, len(streamers))
	streakIdx := make(map[int]struct{})
	hasPriorityGameStreak := false
	for idx, s := range streamers {
		if s == nil || !s.IsOnline {
			continue
		}
		if streamerWatchBackedOff(s, now) {
			continue
		}
		if !s.OnlineAt.IsZero() && now.Sub(s.OnlineAt) < 30*time.Second {
			continue
		}
		m.refreshStreamForPreference(s)
		if s == nil || !s.IsOnline {
			continue
		}
		if m.resolveTimedOutStreak(s, now) {
			m.syncWarmStartCacheFromStreamer(s)
		}
		rank, excluded := m.gamePreference(s)
		if excluded {
			continue
		}
		game := ""
		if s.Stream != nil {
			game = strings.ToLower(strings.TrimSpace(s.Stream.GameName()))
		}
		_, priorityGame := m.gamePriorityIndex[game]
		isStreak := m.shouldPrioritizeStreak(s, now)
		cand := candidate{
			idx:           idx,
			rank:          rank,
			game:          game,
			position:      len(candidates),
			priorityGame:  priorityGame,
			isStreakReady: isStreak,
		}
		candidates = append(candidates, cand)
		candidateByIdx[idx] = cand
		if isStreak {
			streakCandidates = append(streakCandidates, cand)
			streakIdx[idx] = struct{}{}
			if priorityGame {
				hasPriorityGameStreak = true
			}
		}
	}

	sortCandidates := func(list []candidate, less func(a, b candidate) bool, includeGameRank bool) []candidate {
		out := append([]candidate(nil), list...)
		sort.SliceStable(out, func(i, j int) bool {
			a, b := out[i], out[j]
			if less != nil {
				ai := less(a, b)
				aj := less(b, a)
				if ai && !aj {
					return true
				}
				if aj && !ai {
					return false
				}
			}
			if includeGameRank && a.rank != b.rank {
				return a.rank < b.rank
			}
			return a.position < b.position
		})
		return out
	}

	selected := make([]int, 0, maxConcurrentWatchers)
	seen := make(map[int]struct{})
	selectedGames := make(map[string]struct{})
	selectedReason := make(map[int]string)
	add := func(c candidate, reason string, force bool) {
		if len(selected) >= maxConcurrentWatchers {
			return
		}
		if _, ok := seen[c.idx]; ok {
			return
		}
		game := c.game
		if !force && game != "" {
			if _, ok := selectedGames[game]; ok {
				otherAvailable := false
				for _, alt := range candidates {
					if _, picked := seen[alt.idx]; picked {
						continue
					}
					if alt.game != "" && alt.game != game {
						otherAvailable = true
						break
					}
				}
				if otherAvailable {
					return
				}
			}
		}
		seen[c.idx] = struct{}{}
		if game != "" {
			selectedGames[game] = struct{}{}
		}
		selected = append(selected, c.idx)
		if reason != "" {
			selectedReason[c.idx] = reason
		}
	}

	pick := func(list []candidate, includeGameRank bool, less func(a, b candidate) bool, reason string, force bool) {
		for _, c := range sortCandidates(list, less, includeGameRank) {
			add(c, reason, force)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}

	m.activeStreakMu.Lock()
	if len(m.activeStreakWatches) > 0 {
		activeStreaks := make([]candidate, 0, len(m.activeStreakWatches))
		for _, c := range candidates {
			s := streamers[c.idx]
			if s == nil {
				continue
			}
			key := m.activeStreakWatchKey(s)
			watch, ok := m.activeStreakWatches[key]
			if !ok {
				continue
			}
			if !m.shouldKeepActiveStreak(s, now) || !activeStreakWatchMatches(s, watch) {
				delete(m.activeStreakWatches, key)
				continue
			}
			activeStreaks = append(activeStreaks, c)
		}
		for _, c := range sortCandidates(activeStreaks, nil, false) {
			add(c, "ACTIVE_STREAK", true)
			if len(selected) >= maxConcurrentWatchers {
				break
			}
		}
	}
	m.activeStreakMu.Unlock()

	skipEarlyStreak := len(m.gamePriority) > 0 && !hasPriorityGameStreak

	for _, priority := range m.watchPriorities {
		if len(selected) >= maxConcurrentWatchers {
			break
		}
		switch priority {
		case watchPriorityOrder:
			pick(candidates, false, nil, "ORDER", false)
		case watchPriorityStreak:
			if skipEarlyStreak {
				continue
			}
			streaks := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				if m.shouldPrioritizeStreak(streamers[c.idx], now) {
					streaks = append(streaks, c)
				}
			}
			pick(streaks, true, nil, "STREAK", true)
		case watchPriorityDrops:
			drops := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				s := streamers[c.idx]
				if s == nil || s.Stream == nil {
					continue
				}
				// if s.Settings.ClaimDrops && m.streamHasDrops(s) {
				if s.Settings.ClaimDrops {
					drops = append(drops, c)
				}
			}
			pick(drops, true, nil, "DROPS", false)
		case watchPrioritySubscribed:
			subscribed := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				s := streamers[c.idx]
				if s == nil {
					continue
				}
				if s.HasActiveMultipliers() {
					subscribed = append(subscribed, c)
				}
			}
			pick(subscribed, true, func(a, b candidate) bool {
				return streamers[a.idx].TotalMultiplier() > streamers[b.idx].TotalMultiplier()
			}, "SUBSCRIBED", false)
		case watchPriorityPointsAscending:
			asc := append([]candidate(nil), candidates...)
			pick(asc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints < streamers[b.idx].ChannelPoints
			}, "POINTS_ASC", false)
		case watchPriorityPointsDescending:
			desc := append([]candidate(nil), candidates...)
			pick(desc, true, func(a, b candidate) bool {
				return streamers[a.idx].ChannelPoints > streamers[b.idx].ChannelPoints
			}, "POINTS_DESC", false)
		}
	}

	hasStreakSelected := false
	for _, idx := range selected {
		if _, ok := streakIdx[idx]; ok {
			hasStreakSelected = true
			break
		}
	}
	if !hasStreakSelected && len(streakCandidates) > 0 && len(selected) > 0 {
		var streakPick *candidate
		for _, c := range sortCandidates(streakCandidates, nil, true) {
			if _, ok := seen[c.idx]; ok {
				continue
			}
			streakPick = &c
			break
		}
		if streakPick != nil {
			if len(selected) < maxConcurrentWatchers {
				add(*streakPick, "FORCE_STREAK_SLOT2", true)
			} else {
				keepIdx := selected[0]
				selected = selected[:0]
				seen = make(map[int]struct{})
				selectedGames = make(map[string]struct{})
				if keepCand, ok := candidateByIdx[keepIdx]; ok {
					add(keepCand, selectedReason[keepIdx], false)
				}
				if len(selected) < maxConcurrentWatchers {
					if _, ok := seen[streakPick.idx]; !ok {
						seen[streakPick.idx] = struct{}{}
						if streakPick.game != "" {
							selectedGames[streakPick.game] = struct{}{}
						}
						selected = append(selected, streakPick.idx)
						selectedReason[streakPick.idx] = "FORCE_STREAK_SLOT2"
					}
				}
			}
		}
	}

	if skipEarlyStreak && len(selected) >= 2 {
		first := candidateByIdx[selected[0]]
		second := candidateByIdx[selected[1]]
		if first.isStreakReady && !first.priorityGame && (!second.isStreakReady || second.priorityGame) {
			selected[0], selected[1] = selected[1], selected[0]
		}
	}

	if len(selected) < maxConcurrentWatchers {
		pick(candidates, true, nil, "FALLBACK", false)
	}

	watchList := make([]*domain.Streamer, 0, len(selected))
	for _, idx := range selected {
		watchList = append(watchList, streamers[idx])
		m.syncActiveStreakWatch(streamers[idx], now)
	}

	if m.logger != nil && m.logWatchQueue {
		interval := m.watchInterval(len(selected))
		lines := make([]string, 0, len(selected)+1)
		lines = append(lines, fmt.Sprintf("WATCH queue (≈%s between streamers):", formatDuration(interval)))
		for slot, idx := range selected {
			s := streamers[idx]
			cand := candidateByIdx[idx]
			reason := selectedReason[idx]
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

	return watchList
}
