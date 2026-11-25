package miner

// watchSlots is the watch list being built: at most maxConcurrentWatchers streamers,
// with the reason each one was picked.
type watchSlots struct {
	candidates []watchCandidate
	byIdx      map[int]watchCandidate
	picked     []int
	seen       map[int]struct{}
	games      map[string]struct{}
	reasons    map[int]string
}

func newWatchSlots(candidates []watchCandidate) *watchSlots {
	byIdx := make(map[int]watchCandidate, len(candidates))
	for _, c := range candidates {
		byIdx[c.idx] = c
	}
	return &watchSlots{
		candidates: candidates,
		byIdx:      byIdx,
		picked:     make([]int, 0, maxConcurrentWatchers),
		seen:       make(map[int]struct{}),
		games:      make(map[string]struct{}),
		reasons:    make(map[int]string),
	}
}

func (w *watchSlots) full() bool {
	return len(w.picked) >= maxConcurrentWatchers
}

// add gives c a slot. Unless forced, a second streamer of an already picked game
// is skipped while a streamer of another game is still available.
func (w *watchSlots) add(c watchCandidate, reason string, force bool) {
	if w.full() {
		return
	}
	if _, ok := w.seen[c.idx]; ok {
		return
	}
	game := c.game
	if !force && game != "" {
		if _, ok := w.games[game]; ok {
			otherAvailable := false
			for _, alt := range w.candidates {
				if _, picked := w.seen[alt.idx]; picked {
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
	w.take(c, reason)
}

// take records c as picked without any checks.
func (w *watchSlots) take(c watchCandidate, reason string) {
	w.seen[c.idx] = struct{}{}
	if c.game != "" {
		w.games[c.game] = struct{}{}
	}
	w.picked = append(w.picked, c.idx)
	if reason != "" {
		w.reasons[c.idx] = reason
	}
}

// pick adds candidates from list in sorted order until the slots are full.
func (w *watchSlots) pick(list []watchCandidate, includeGameRank bool, less func(a, b watchCandidate) bool, reason string, force bool) {
	for _, c := range sortWatchCandidates(list, less, includeGameRank) {
		w.add(c, reason, force)
		if w.full() {
			break
		}
	}
}

// ensureStreak makes sure a streamer with a pending watch streak holds a slot when
// one exists: it takes a free slot, or replaces the second pick.
func (w *watchSlots) ensureStreak(streakReady []watchCandidate) {
	if len(streakReady) == 0 || len(w.picked) == 0 {
		return
	}
	for _, idx := range w.picked {
		if w.byIdx[idx].isStreakReady {
			return
		}
	}
	var streakPick watchCandidate
	found := false
	for _, c := range sortWatchCandidates(streakReady, nil, true) {
		if _, ok := w.seen[c.idx]; ok {
			continue
		}
		streakPick, found = c, true
		break
	}
	if !found {
		return
	}
	const reason = "FORCE_STREAK_SLOT2"
	if !w.full() {
		w.add(streakPick, reason, true)
		return
	}
	keepIdx := w.picked[0]
	w.picked = w.picked[:0]
	w.seen = make(map[int]struct{})
	w.games = make(map[string]struct{})
	if keep, ok := w.byIdx[keepIdx]; ok {
		w.add(keep, w.reasons[keepIdx], false)
	}
	if !w.full() {
		if _, ok := w.seen[streakPick.idx]; !ok {
			w.take(streakPick, reason)
		}
	}
}

// preferPriorityGameFirst moves a streak on a non-priority game behind the other pick.
func (w *watchSlots) preferPriorityGameFirst() {
	if len(w.picked) < 2 {
		return
	}
	first := w.byIdx[w.picked[0]]
	second := w.byIdx[w.picked[1]]
	if first.isStreakReady && !first.priorityGame && (!second.isStreakReady || second.priorityGame) {
		w.picked[0], w.picked[1] = w.picked[1], w.picked[0]
	}
}
