package miner

import (
	"sort"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

// watchCandidate is one online streamer that may be watched this round.
type watchCandidate struct {
	idx           int
	rank          int
	game          string
	position      int
	priorityGame  bool
	isStreakReady bool
}

// watchCandidates lists the streamers that can be watched right now, in list order.
// It refreshes each one's stream first, so game preferences and streak state are current.
func (m *Miner) watchCandidates(streamers []*domain.Streamer, now time.Time) []watchCandidate {
	candidates := make([]watchCandidate, 0, len(streamers))
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
		candidates = append(candidates, watchCandidate{
			idx:           idx,
			rank:          rank,
			game:          game,
			position:      len(candidates),
			priorityGame:  priorityGame,
			isStreakReady: m.shouldPrioritizeStreak(s, now),
		})
	}
	return candidates
}

func streakReadyCandidates(candidates []watchCandidate) []watchCandidate {
	ready := make([]watchCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.isStreakReady {
			ready = append(ready, c)
		}
	}
	return ready
}

func anyPriorityGame(candidates []watchCandidate) bool {
	for _, c := range candidates {
		if c.priorityGame {
			return true
		}
	}
	return false
}

// sortWatchCandidates orders a copy of list by less, then by game rank when asked,
// then by position in the streamer list.
func sortWatchCandidates(list []watchCandidate, less func(a, b watchCandidate) bool, includeGameRank bool) []watchCandidate {
	out := append([]watchCandidate(nil), list...)
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
