package twitchchannelpointsminer

import (
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func testWatchSelectionStreakStreamer(username, channelID, game string, onlineAt time.Time, minuteWatched float64) *entities.Streamer {
	return &entities.Streamer{
		Username:  username,
		ChannelID: channelID,
		IsOnline:  true,
		OnlineAt:  onlineAt,
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			BroadcastID:        username + "-broadcast",
			Game:               map[string]interface{}{"displayName": game},
			WatchStreakMissing: true,
			MinuteWatched:      minuteWatched,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}
}

func TestParseWatchPriorities(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []watchPriority
	}{
		{
			name: "defaults when empty",
			in:   nil,
			want: defaultWatchPriorities(),
		},
		{
			name: "ignores unknowns and deduplicates",
			in:   []string{"drops", "ORDER", "drops", "points_desc", "ignored"},
			want: []watchPriority{
				watchPriorityDrops,
				watchPriorityOrder,
				watchPriorityPointsDescending,
			},
		},
		{
			name: "falls back to defaults when nothing recognized",
			in:   []string{"foo", "bar"},
			want: defaultWatchPriorities(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWatchPriorities(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d got %v want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeGameList(t *testing.T) {
	got := normalizeGameList([]string{" Valorant ", "Tom Clancy's Rainbow Six Siege X", "five night's at freddys", "valorant", "   "})
	want := []string{"valorant", "tom clancy's rainbow six siege x", "five night's at freddys"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestWatchInterval(t *testing.T) {
	m := &Miner{}
	if got := m.watchInterval(0); got != 20*time.Second {
		t.Fatalf("zero count got %s", got)
	}
	if got := m.watchInterval(2); got != 10*time.Second {
		t.Fatalf("two watchers got %s", got)
	}
	if got := m.watchInterval(10); got != 5*time.Second {
		t.Fatalf("min interval cap expected 5s got %s", got)
	}
}

func TestStreakPriorityLimit(t *testing.T) {
	m := &Miner{}
	now := time.Now()
	if got := m.streakPriorityLimit(now); got != streakPriorityMinutesBase {
		t.Fatalf("zero start uses base got %f", got)
	}
	m.startedAt = now.Add(-11 * time.Hour)
	if got := m.streakPriorityLimit(now); got != streakPriorityMinutesExtended {
		t.Fatalf("long session uses extended got %f", got)
	}
}

func TestShouldPrioritizeStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
		},
	}
	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("expected streak priority when conditions met")
	}

	streamer.OfflineAt = now.Add(-10 * time.Minute)
	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("recent offline should skip streak priority")
	}

	streamer.OfflineAt = time.Time{}
	streamer.Stream.WatchStreakMissing = false
	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("resolved streak should not keep streak priority")
	}
}

func TestShouldPrioritizeStreakAllowsRestartedStreamWithinOfflineCooldown(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)

	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         now.Add(-9 * time.Minute),
			CreatedAt:          now.Add(-9 * time.Minute),
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("restarted stream should regain streak priority inside the offline cooldown")
	}
}

func TestShouldPrioritizeStreakAllowsStreamStartedJustBeforeFalseOffline(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)
	streamStartedAt := offlineAt.Add(-20 * time.Second)

	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("stream started just before a false offline event should regain streak priority")
	}
}

func TestShouldPrioritizeStreakBlocksStreamStartedWellBeforeRecentOffline(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-10 * time.Minute)
	streamStartedAt := offlineAt.Add(-5 * time.Minute)

	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		OfflineAt: offlineAt,
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("stream started well before a recent offline event should stay blocked by cooldown")
	}
}

func TestResolvedStreakShortRestartDoesNotPrioritizeStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}
	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved short restart should not keep streak priority")
	}
}

func TestResolveTimedOutStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      streakPriorityMinutesBase,
		},
	}

	if !m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("expected timed-out streak to resolve")
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("timed-out streak should clear pending state")
	}
	if m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("resolved streak should not be resolved twice")
	}
}

func TestResolveTimedOutStreakShortRestartStillPrioritizesStreak(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "timedout",
		ChannelID:     "channel-timedout",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      streakPriorityMinutesBase,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	if !m.resolveTimedOutStreak(streamer, now) {
		t.Fatalf("expected timed-out streak to resolve before restart")
	}

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if !m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("timed-out short restart should still prioritize streak")
	}
}

func TestResolvedStreakShortRestartCarryoverSurvivesStreamRefreshReset(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved short restart should stay suppressed after stream refresh reset")
	}
}

func TestResolvedStreakShortRestartCarryoverLearnsCompletionFromUpdateSync(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	streamer.Stream.WatchStreakMissing = false
	m.syncResolvedStreakState(streamer, streamer.Stream.BroadcastID, streamer.Stream.CreatedAt, true)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("completion learned from update sync should suppress short restart streak priority")
	}
}

func TestResolvedStreakShortRestartCarryoverSurvivesMultipleRestartCycles(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}
	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	if firstCarryoverUntil.IsZero() {
		t.Fatalf("first restart should arm a carryover window")
	}
	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	m.setPresence(streamer, false, "poll")
	secondOfflineAt := streamer.OfflineAt
	secondCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	wantSecondCarryoverUntil := secondOfflineAt.Add(resolvedStreakCarryoverWindow)
	if !secondCarryoverUntil.Equal(wantSecondCarryoverUntil) {
		t.Fatalf("repeated short restart should extend carryover to latest offline time, got %s want %s", secondCarryoverUntil, wantSecondCarryoverUntil)
	}

	streamer.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("carryover should continue suppressing streak priority across repeated short restarts")
	}
}

func TestResolvedStreakCarryoverPropagatesAfterOriginalExpiryAcrossShortRestarts(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)

	m.setPresence(streamer, false, "poll")
	streamer.Stream = &entities.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	streamer.ResolvedStreakCarryoverUntil = time.Now().Add(-time.Second)

	m.setPresence(streamer, false, "poll")
	if streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("resolved carryover should re-arm from the later offline segment")
	}

	streamer.Stream = &entities.Stream{
		BroadcastID:        "broadcast-c",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(2 * time.Minute),
		CreatedAt:          now.Add(2 * time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("resolved carryover should suppress streak priority after repeated short restarts")
	}
}

func TestRememberResolvedStreakCarryoverExtendsActiveWindowFromLatestOfflineTime(t *testing.T) {
	firstOfflineAt := time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)
	secondOfflineAt := firstOfflineAt.Add(5 * time.Minute)
	streamer := &entities.Streamer{
		CompletedWatchStreak: true,
		OfflineAt:            firstOfflineAt,
	}

	rememberResolvedStreakCarryover(streamer, firstOfflineAt)
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil

	streamer.OfflineAt = secondOfflineAt
	rememberResolvedStreakCarryover(streamer, secondOfflineAt)

	want := secondOfflineAt.Add(resolvedStreakCarryoverWindow)
	if !streamer.ResolvedStreakCarryoverUntil.Equal(want) {
		t.Fatalf("carryover expiry got %s want %s", streamer.ResolvedStreakCarryoverUntil, want)
	}
	if !streamer.ResolvedStreakCarryoverUntil.After(firstCarryoverUntil) {
		t.Fatalf("carryover expiry should move forward, got %s first %s", streamer.ResolvedStreakCarryoverUntil, firstCarryoverUntil)
	}
}

func TestResolvedStreakCarryoverExpiresAfterWindow(t *testing.T) {
	now := time.Now()
	m := &Miner{}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		ResolvedStreakCarryover:      true,
		ResolvedStreakCarryoverUntil: now.Add(-time.Second),
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-10 * time.Minute),
			CreatedAt:          now.Add(-10 * time.Minute),
		},
	}

	if !m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("expired carryover should no longer suppress streak priority")
	}
	if streamer.ResolvedStreakCarryover {
		t.Fatalf("expired carryover should be cleared")
	}
	if !streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("expired carryover timestamp should be cleared")
	}
}

func TestActivateResolvedStreakCarryoverUsesRecentOfflineTime(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-20 * time.Second)
	streamer := &entities.Streamer{
		OfflineAt: offlineAt,
	}

	activateResolvedStreakCarryover(streamer, now)

	want := offlineAt.Add(resolvedStreakCarryoverWindow)
	if !streamer.ResolvedStreakCarryoverUntil.Equal(want) {
		t.Fatalf("carryover expiry got %s want %s", streamer.ResolvedStreakCarryoverUntil, want)
	}
}

func TestResolvedStreakCarryoverArmsAcrossOnlineBroadcastReset(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			BroadcastID:        "broadcast-2",
			WatchStreakMissing: true,
			MinuteWatched:      0,
			StreamUpAt:         now.Add(-time.Minute),
			CreatedAt:          now.Add(-time.Minute),
		},
	}
	streamer.CompletedWatchStreak = true

	m.syncResolvedStreakState(streamer, "broadcast-1", now.Add(-15*time.Minute), true)

	if m.shouldPrioritizeStreak(streamer, now) {
		t.Fatalf("online broadcast reset after a completed segment should suppress streak priority")
	}
	if !streamer.ResolvedStreakCarryover {
		t.Fatalf("online broadcast reset should arm carryover")
	}
	if streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("online broadcast reset should set carryover expiry")
	}
}

func TestResolvedStreakCarryoverLongOfflineClearsPropagation(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)
	m.setPresence(streamer, false, "poll")
	streamer.ResolvedStreakCarryoverUntil = time.Now().Add(-time.Second)

	streamer.Stream = &entities.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(streamer, true, "poll")
	streamer.OnlineAt = now.Add(-time.Minute)

	if streamer.CompletedWatchStreak {
		t.Fatalf("long offline gap should clear propagation signal")
	}
	if streamer.ResolvedStreakCarryover {
		t.Fatalf("expired carryover should stay cleared after long offline gap")
	}
	if !streamer.ResolvedStreakCarryoverUntil.IsZero() {
		t.Fatalf("expired carryover timestamp should stay cleared after long offline gap")
	}
	if !m.shouldPrioritizeStreak(streamer, time.Now()) {
		t.Fatalf("new segment after long offline gap should be eligible for streak priority")
	}
}

func TestResolvedStreakCarryoverRefreshesFromFreshCompletionDuringActiveWindow(t *testing.T) {
	now := time.Now()
	m := &Miner{logger: discardLogger()}
	streamer := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      now.Add(-time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			BroadcastID:        "broadcast-a",
			WatchStreakMissing: true,
			MinuteWatched:      2,
			StreamUpAt:         now.Add(-15 * time.Minute),
			CreatedAt:          now.Add(-15 * time.Minute),
		},
	}

	m.updateHistory(streamer, "WATCH_STREAK", 50)
	m.setPresence(streamer, false, "poll")
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil
	if firstCarryoverUntil.IsZero() {
		t.Fatalf("segment A should arm carryover")
	}

	streamer.Stream = &entities.Stream{
		BroadcastID:        "broadcast-b",
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(-time.Minute),
		CreatedAt:          now.Add(-time.Minute),
	}
	m.setPresence(streamer, true, "poll")

	m.updateHistory(streamer, "WATCH_STREAK", 75)
	if !streamer.CompletedWatchStreak {
		t.Fatalf("segment B should record a fresh completion")
	}

	time.Sleep(20 * time.Millisecond)
	m.setPresence(streamer, false, "poll")
	if !streamer.CompletedWatchStreak {
		t.Fatalf("fresh completion should keep propagation active when carryover is refreshed")
	}
	if !streamer.ResolvedStreakCarryoverUntil.After(firstCarryoverUntil) {
		t.Fatalf("fresh completion should refresh carryover expiry, got %s want after %s", streamer.ResolvedStreakCarryoverUntil, firstCarryoverUntil)
	}
}

func TestGamePreference(t *testing.T) {
	m := &Miner{
		gamePriority:      []string{"valorant", "wow"},
		gamePriorityIndex: map[string]int{"valorant": 0, "wow": 1},
		gameExclusions:    map[string]struct{}{"banned": {}},
	}
	streamer := &entities.Streamer{
		Stream: &entities.Stream{
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

func TestGameInfoRespectsShowFlag(t *testing.T) {
	streamer := &entities.Streamer{
		Stream: &entities.Stream{
			Game: map[string]interface{}{"displayName": "Tom Clancy's Rainbow Six Siege X"},
		},
	}
	m := &Miner{showGameInfo: false}
	if got := m.gameInfo(streamer); got != "" {
		t.Fatalf("expected no game info when disabled got %q", got)
	}

	m.showGameInfo = true
	if got := m.gameInfo(streamer); got != "Tom Clancy's Rainbow Six Siege X" {
		t.Fatalf("expected game name when enabled got %q", got)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatChannelPoints(999); got != "999" {
		t.Fatalf("channel points base got %s", got)
	}
	if got := formatChannelPoints(1500); got != "1.5k" {
		t.Fatalf("channel points thousands got %s", got)
	}
	if got := formatChannelPoints(1500000); got != "1.5M" {
		t.Fatalf("channel points millions got %s", got)
	}

	if got := formatPointsWithSuffix(1250000, 1_000_000, "M"); got != "1.25M" {
		t.Fatalf("suffix format got %s", got)
	}

	if got := formatDropProgress(3, 10); got != "3/10" {
		t.Fatalf("drop progress got %s", got)
	}
	if got := formatDropProgress(5, 0); got != "5" {
		t.Fatalf("drop progress with zero required got %s", got)
	}

	if got := progressPercent(5, 10); got != 50 {
		t.Fatalf("percent got %d", got)
	}
	if got := progressPercent(1, 0); got != 100 {
		t.Fatalf("zero required but progress present got %d", got)
	}
	if got := progressPercent(0, 0); got != 0 {
		t.Fatalf("zero current and required got %d", got)
	}
}

func TestFormatDuration(t *testing.T) {
	dur := 24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second
	if got := formatDuration(dur); got != "1d 02h 03m 04s" {
		t.Fatalf("duration format got %q", got)
	}
	if got := formatDuration(45 * time.Second); got != "45s" {
		t.Fatalf("short duration format got %q", got)
	}
}

func TestNewSessionIDShape(t *testing.T) {
	id := newSessionID()
	if id == "" {
		t.Fatalf("id should not be empty")
	}
	if strings.Count(id, "-") != 4 {
		t.Fatalf("expected 4 hyphens got %d (%s)", strings.Count(id, "-"), id)
	}
}

func TestSleepWithStop(t *testing.T) {
	m := &Miner{}
	stop := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		done <- m.sleepWithStop(150*time.Millisecond, stop)
	}()
	time.Sleep(20 * time.Millisecond)
	close(stop)
	if !<-done {
		t.Fatalf("expected sleep to abort on stop")
	}

	start := time.Now()
	if m.sleepWithStop(50*time.Millisecond, make(chan struct{})) {
		t.Fatalf("expected no stop signal")
	}
	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("sleep returned too early: %s", elapsed)
	}
}

func TestPickStreamersToWatchSkipsSubscribedPriorityWhenNone(t *testing.T) {
	onlineAt := time.Now().Add(-time.Minute)
	streamers := []*entities.Streamer{
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

	first := &entities.Streamer{
		Username:      "first",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 100,
	}
	second := &entities.Streamer{
		Username:      "second",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 200,
	}
	restarted := &entities.Streamer{
		Username:  "restarted",
		IsOnline:  true,
		OnlineAt:  onlineAt,
		OfflineAt: now.Add(-10 * time.Minute),
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         now.Add(-9 * time.Minute),
			CreatedAt:          now.Add(-9 * time.Minute),
		},
	}

	m := &Miner{
		watchPriorities: defaultWatchPriorities(),
	}

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, restarted})
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

	first := &entities.Streamer{
		Username:      "first",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 100,
	}
	second := &entities.Streamer{
		Username:      "second",
		IsOnline:      true,
		OnlineAt:      onlineAt,
		ChannelPoints: 200,
	}
	restarted := &entities.Streamer{
		Username:  "restarted",
		IsOnline:  true,
		OnlineAt:  onlineAt,
		OfflineAt: offlineAt,
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
			WatchStreakMissing: true,
			MinuteWatched:      3,
			StreamUpAt:         streamStartedAt,
			CreatedAt:          streamStartedAt,
		},
	}

	m := &Miner{
		watchPriorities: defaultWatchPriorities(),
	}

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, restarted})
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

	first := &entities.Streamer{
		Username: "first",
		IsOnline: true,
		OnlineAt: onlineAt,
	}
	second := &entities.Streamer{
		Username: "second",
		IsOnline: true,
		OnlineAt: onlineAt,
	}
	restarted := &entities.Streamer{
		Username:      "restarted",
		ChannelID:     "channel-restarted",
		IsOnline:      true,
		PresenceKnown: true,
		OnlineAt:      onlineAt,
		Settings: entities.StreamerSettings{
			WatchStreak: true,
		},
		Stream: &entities.Stream{
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
	restarted.Stream = &entities.Stream{
		WatchStreakMissing: true,
		MinuteWatched:      0,
		StreamUpAt:         now.Add(time.Minute),
		CreatedAt:          now.Add(time.Minute),
	}
	m.setPresence(restarted, true, "poll")
	restarted.OnlineAt = onlineAt

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, restarted})
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

	initial := m.pickStreamersToWatch([]*entities.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}
	if initial[0] != first || initial[1] != second {
		t.Fatalf("expected initial in-progress streak selection to stay in order, got %s then %s", initial[0].Username, initial[1].Username)
	}

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, higherPriority})
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

	initial := m.pickStreamersToWatch([]*entities.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, higherPriority})
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

	initial := m.pickStreamersToWatch([]*entities.Streamer{first, second})
	if len(initial) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(initial))
	}

	first.Stream.WatchStreakMissing = false

	got := m.pickStreamersToWatch([]*entities.Streamer{first, second, higherPriority})
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
	fallback := &entities.Streamer{
		Username: "fallback",
		IsOnline: true,
		OnlineAt: onlineAt,
		Stream: &entities.Stream{
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

	got := m.pickStreamersToWatch([]*entities.Streamer{active, sameGameStreak, fallback})
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

	subscribed := &entities.Streamer{
		Username: "subscribed",
		IsOnline: true,
		OnlineAt: onlineAt,
		ActiveMultipliers: []entities.ActiveMultiplier{
			{Factor: 1.2},
		},
		Stream: &entities.Stream{
			BroadcastID:        "subscribed-broadcast",
			Game:               map[string]interface{}{"displayName": "Just Chatting"},
			WatchStreakMissing: false,
			CreatedAt:          onlineAt,
			StreamUpAt:         onlineAt,
		},
	}
	sameGameStreak := testWatchSelectionStreakStreamer("same-game", "channel-same-game", "Just Chatting", onlineAt, 0)
	fallback := &entities.Streamer{
		Username: "fallback",
		IsOnline: true,
		OnlineAt: onlineAt,
		Stream: &entities.Stream{
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

	got := m.pickStreamersToWatch([]*entities.Streamer{subscribed, sameGameStreak, fallback})
	if len(got) != 2 {
		t.Fatalf("expected 2 watchers got %d", len(got))
	}
	if got[0] != subscribed || got[1] != sameGameStreak {
		t.Fatalf("expected forced streak slot to outrank fallback, got %s then %s", got[0].Username, got[1].Username)
	}
}

func TestSyncActiveStreakWatchTracksOnlyEligibleInProgressStreaks(t *testing.T) {
	now := time.Now()
	streamer := testWatchSelectionStreakStreamer("first", "channel-first", "casual game", now.Add(-time.Minute), 0)
	m := &Miner{}

	m.syncActiveStreakWatch(streamer, now)
	if _, ok := m.activeStreakWatches[streamer.ChannelID]; ok {
		t.Fatalf("fresh streak without progress should not be tracked")
	}

	streamer.Stream.MinuteWatched = 1.25
	m.syncActiveStreakWatch(streamer, now)
	watch, ok := m.activeStreakWatches[streamer.ChannelID]
	if !ok {
		t.Fatalf("eligible streak should be tracked")
	}
	if watch.BroadcastID != streamer.Stream.BroadcastID {
		t.Fatalf("tracked broadcast mismatch got %q want %q", watch.BroadcastID, streamer.Stream.BroadcastID)
	}

	streamer.Stream.WatchStreakMissing = false
	m.syncActiveStreakWatch(streamer, now)
	if _, ok := m.activeStreakWatches[streamer.ChannelID]; ok {
		t.Fatalf("resolved streak should be removed from active tracking")
	}
}

func TestFilterExcludedTargets(t *testing.T) {
	m := &Miner{
		streamerExclusions: map[string]struct{}{
			"foo": {},
			"bar": {},
		},
	}

	got, excluded := m.filterExcludedTargets([]string{"Foo", "baz", "  bar  ", "", "qux"})
	want := []string{"baz", "qux"}

	if excluded != 2 {
		t.Fatalf("expected 2 excluded got %d", excluded)
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestShouldClaimDrops(t *testing.T) {
	tests := []struct {
		name      string
		global    bool
		streamers []*entities.Streamer
		want      bool
	}{
		{
			name:   "enabled globally",
			global: true,
			want:   true,
		},
		{
			name:   "enabled by streamer override",
			global: false,
			streamers: []*entities.Streamer{
				{Settings: entities.StreamerSettings{ClaimDrops: false}},
				{Settings: entities.StreamerSettings{ClaimDrops: true}},
			},
			want: true,
		},
		{
			name:   "disabled globally and per streamer",
			global: false,
			streamers: []*entities.Streamer{
				{Settings: entities.StreamerSettings{ClaimDrops: false}},
				nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Miner{
				StreamerSettings: entities.StreamerSettings{
					ClaimDrops: tt.global,
				},
			}
			if got := m.shouldClaimDrops(tt.streamers); got != tt.want {
				t.Fatalf("shouldClaimDrops() = %t, want %t", got, tt.want)
			}
		})
	}
}
