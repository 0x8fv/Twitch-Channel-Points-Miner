package miner

import (
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func testWatchSelectionStreakStreamer(username, channelID, game string, onlineAt time.Time, minuteWatched float64) *domain.Streamer {
	return &domain.Streamer{
		Username:  username,
		ChannelID: channelID,
		IsOnline:  true,
		OnlineAt:  onlineAt,
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			BroadcastID:        username + "-broadcast",
			Game:               map[string]interface{}{"displayName": game},
			WatchStreakMissing: true,
			MinuteWatched:      minuteWatched,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}
}

func TestGamePreference(t *testing.T) {
	m := &Miner{
		gamePriority:      []string{"valorant", "wow"},
		gamePriorityIndex: map[string]int{"valorant": 0, "wow": 1},
		gameExclusions:    map[string]struct{}{"banned": {}},
	}
	streamer := &domain.Streamer{
		Stream: &domain.Stream{
			Game: map[string]interface{}{"displayName": "Valorant"},
		},
	}
	rank, excluded := m.gamePreference(streamer)
	if excluded {
		t.Fatalf("expected not excluded")
	}
	if rank != 0 {
		t.Fatalf("expected rank 0 got %d", rank)
	}

	streamer.Stream.Game["displayName"] = "Banned"
	rank, excluded = m.gamePreference(streamer)
	if !excluded || rank != 0 {
		t.Fatalf("excluded game should return excluded=true rank=0 got rank=%d excluded=%t", rank, excluded)
	}
}

func TestPickStreamersToWatchSkipsSubscribedPriorityWhenNone(t *testing.T) {
	onlineAt := time.Now().Add(-time.Minute)
	streamers := []*domain.Streamer{
		{Username: "streamer1", IsOnline: true, OnlineAt: onlineAt, ChannelPoints: 1000},
		{Username: "streamer8", IsOnline: true, OnlineAt: onlineAt, ChannelPoints: 900},
		{Username: "streamer9", IsOnline: true, OnlineAt: onlineAt, ChannelPoints: 10},
		{Username: "streamer10", IsOnline: true, OnlineAt: onlineAt, ChannelPoints: 20},
	}

	m := &Miner{
		watchPriorities: parseWatchPriorities([]string{"STREAK", "SUBSCRIBED", "POINTS_ASC"}),
	}

	got := m.pickStreamersToWatch(streamers)
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != streamers[2] || got[1] != streamers[3] {
		t.Fatalf("expected lowest points prioritized, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchPrioritizesRestartedStreakCandidate(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	first := &domain.Streamer{
		Username:      "first",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 100,
	}
	second := &domain.Streamer{
		Username:      "second",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 200,
	}
	restarted := &domain.Streamer{
		Username:  "restarted",
		IsOnline:  true,
		OnlineAt:  onlineAt,
		OfflineAt: now.Add(-10 * time.Minute),
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         now.Add(-9 * time.Minute),
			CreatedAt:          now.Add(-9 * time.Minute),
		},
	}

	m := &Miner{
		watchPriorities: defaultWatchPriorities(),
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, restarted})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != restarted {
		t.Fatalf("expected restarted streak candidate to take the first slot, got %s", got[0].Username)
	}
	if got[1] != first {
		t.Fatalf("expected the next order candidate to fill slot two, got %s", got[1].Username)
	}
}

func TestPickStreamersToWatchPrioritizesStreamStartedJustBeforeFalseOffline(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)
	offlineAt := now.Add(-10 * time.Minute)
	streamStartedAt := offlineAt.Add(-20 * time.Second)

	first := &domain.Streamer{
		Username:      "first",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 100,
	}
	second := &domain.Streamer{
		Username:      "second",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 200,
	}
	restarted := &domain.Streamer{
		Username:  "restarted",
		IsOnline:  true,
		OnlineAt:  onlineAt,
		OfflineAt: offlineAt,
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	m := &Miner{
		watchPriorities: defaultWatchPriorities(),
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, restarted})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != restarted {
		t.Fatalf("expected false-offline restarted streak candidate to take the first slot, got %s", got[0].Username)
	}
	if got[1] != first {
		t.Fatalf("expected the next order candidate to fill slot two, got %s", got[1].Username)
	}
}

func TestPickStreamersToWatchDoesNotPrioritizeRestartedResolvedStreak(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	first := &domain.Streamer{
		Username: "first",
		IsOnline: true,
		OnlineAt: onlineAt,
	}
	second := &domain.Streamer{
		Username: "second",
		IsOnline: true,
		OnlineAt: onlineAt,
	}
	restarted := &domain.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      onlineAt,
		Settings: domain.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &domain.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m := &Miner{
		logger:          discardLogger(),
		watchPriorities: defaultWatchPriorities(),
	}
	m.updateHistory(restarted, "WATCH_STREAK", 50)

	m.setPresence(restarted, false, "poll")
	restarted.Stream = &domain.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(restarted, true, "poll")
	restarted.OnlineAt = onlineAt

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, restarted})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != first || got[1] != second {
		t.Fatalf("expected resolved restarted streak to stay out of streak priority, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchKeepsInProgressStreakSlotsWhenHigherPriorityStreakAppears(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	first := testWatchSelectionStreakStreamer("first", "channel-first", "casual game", onlineAt, 1.5)
	second := testWatchSelectionStreakStreamer("second", "channel-second", "another game", onlineAt, 2.0)
	higherPriority := testWatchSelectionStreakStreamer("priority", "channel-priority", "priority game", onlineAt, 0)

	m := &Miner{
		watchPriorities:   defaultWatchPriorities(),
		gamePriority:      []string{"priority game"},
		gamePriorityIndex: map[string]int{"priority game": 0},
	}

	initial := m.pickStreamersToWatch([]*domain.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}
	if initial[0] != first || initial[1] != second {
		t.Fatalf("expected initial in-progress streak selection to stay in order, got %s then %s", initial[0].Username, initial[1].Username)
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, higherPriority})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != first || got[1] != second {
		t.Fatalf("expected in-progress streaks to keep both slots, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchDoesNotPinFreshStreaksWithoutProgress(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	first := testWatchSelectionStreakStreamer("first", "channel-first", "casual game", onlineAt, 0)
	second := testWatchSelectionStreakStreamer("second", "channel-second", "another game", onlineAt, 0)
	higherPriority := testWatchSelectionStreakStreamer("priority", "channel-priority", "priority game", onlineAt, 0)

	m := &Miner{
		watchPriorities:   defaultWatchPriorities(),
		gamePriority:      []string{"priority game"},
		gamePriorityIndex: map[string]int{"priority game": 0},
	}

	initial := m.pickStreamersToWatch([]*domain.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, higherPriority})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != higherPriority || got[1] != first {
		t.Fatalf("expected fresh streaks to remain pre-emptible, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchReleasesResolvedActiveStreakSlot(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	first := testWatchSelectionStreakStreamer("first", "channel-first", "casual game", onlineAt, 1.5)
	second := testWatchSelectionStreakStreamer("second", "channel-second", "another game", onlineAt, 2.0)
	higherPriority := testWatchSelectionStreakStreamer("priority", "channel-priority", "priority game", onlineAt, 0)

	m := &Miner{
		watchPriorities:   defaultWatchPriorities(),
		gamePriority:      []string{"priority game"},
		gamePriorityIndex: map[string]int{"priority game": 0},
	}

	initial := m.pickStreamersToWatch([]*domain.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}

	first.Stream.WatchStreakMissing = false

	got := m.pickStreamersToWatch([]*domain.Streamer{first, second, higherPriority})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != second || got[1] != higherPriority {
		t.Fatalf("expected resolved streak slot to release, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchKeepsSameGameStreakAheadOfFallback(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	active := testWatchSelectionStreakStreamer("active", "channel-active", "Just Chatting", onlineAt, 2.0)
	sameGameStreak := testWatchSelectionStreakStreamer("same-game", "channel-same-game", "Just Chatting", onlineAt, 0)
	fallback := &domain.Streamer{
		Username: "fallback",
		IsOnline: true,
		OnlineAt: onlineAt,
		Stream: &domain.Stream{
			BroadcastID:        "fallback-broadcast",
			Game:               map[string]interface{}{"displayName": "Different Game"},
			WatchStreakMissing: false,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}

	m := &Miner{
		watchPriorities: []watchPriority{watchPriorityStreak},
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{active, sameGameStreak, fallback})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != active || got[1] != sameGameStreak {
		t.Fatalf("expected same-game streak to outrank fallback, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchForceStreakSlotIgnoresGameDiversity(t *testing.T) {
	now := time.Now()
	onlineAt := now.Add(-time.Minute)

	subscribed := &domain.Streamer{
		Username: "subscribed",
		IsOnline: true,
		OnlineAt: onlineAt,
		ActiveMultipliers: []domain.ActiveMultiplier{
			{Factor: 1.2},
		},
		Stream: &domain.Stream{
			BroadcastID:        "subscribed-broadcast",
			Game:               map[string]interface{}{"displayName": "Just Chatting"},
			WatchStreakMissing: false,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}
	sameGameStreak := testWatchSelectionStreakStreamer("same-game", "channel-same-game", "Just Chatting", onlineAt, 0)
	fallback := &domain.Streamer{
		Username: "fallback",
		IsOnline: true,
		OnlineAt: onlineAt,
		Stream: &domain.Stream{
			BroadcastID:        "fallback-broadcast",
			Game:               map[string]interface{}{"displayName": "Different Game"},
			WatchStreakMissing: false,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}

	m := &Miner{
		watchPriorities: []watchPriority{watchPrioritySubscribed},
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{subscribed, sameGameStreak, fallback})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != subscribed || got[1] != sameGameStreak {
		t.Fatalf("expected forced streak slot to outrank fallback, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestPickStreamersToWatchSkipsBackedOffStreamer(t *testing.T) {
	now := time.Now()
	backedOff := testWatchSelectionStreakStreamer("stuck", "channel-stuck", "game", now.Add(-time.Minute), 0)
	backedOff.WatchBackoffUntil = now.Add(10 * time.Minute)
	backedOff.WatchFailureCount = 3

	healthy := testWatchSelectionStreakStreamer("healthy", "channel-healthy", "game", now.Add(-time.Minute), 0)
	fallback := &domain.Streamer{
		Username: "fallback",
		IsOnline: true,
		OnlineAt: now.Add(-time.Minute),
		Stream: &domain.Stream{
			BroadcastID:        "fallback-broadcast",
			Game:               map[string]interface{}{"displayName": "Different Game"},
			WatchStreakMissing: false,
			CreatedAt:          now.Add(-time.Minute),
			StreamUpAt:         now.Add(-time.Minute),
		},
	}

	m := &Miner{
		watchPriorities: []watchPriority{watchPriorityStreak, watchPriorityOrder},
	}

	got := m.pickStreamersToWatch([]*domain.Streamer{backedOff, healthy, fallback})
	for _, streamer := range got {
		if streamer == backedOff {
			t.Fatalf("backed off streamer should not be selected")
		}
	}
	if len(got) == 0 || got[0] != healthy {
		t.Fatalf("healthy streak should still be selectable")
	}
}
