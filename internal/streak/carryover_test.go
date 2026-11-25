package streak

import (
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestRememberResolvedStreakCarryoverExtendsActiveWindowFromLatestOfflineTime(t *testing.T) {
	firstOfflineAt := time.Date(2026, time.April, 30, 12, 0, 0, 0, time.UTC)
	secondOfflineAt := firstOfflineAt.Add(5 * time.Minute)
	streamer := &domain.Streamer{
		CompletedWatchStreak: true,
		OfflineAt:            firstOfflineAt,
	}

	RememberCarryover(streamer, firstOfflineAt)
	firstCarryoverUntil := streamer.ResolvedStreakCarryoverUntil

	streamer.OfflineAt = secondOfflineAt
	RememberCarryover(streamer, secondOfflineAt)

	want := secondOfflineAt.Add(CarryoverWindow)
	if !streamer.ResolvedStreakCarryoverUntil.Equal(want) {
		t.Fatalf("carryover expiry got %s want %s", streamer.ResolvedStreakCarryoverUntil, want)
	}
	if !streamer.ResolvedStreakCarryoverUntil.After(firstCarryoverUntil) {
		t.Fatalf("carryover expiry should move forward, got %s first %s", streamer.ResolvedStreakCarryoverUntil, firstCarryoverUntil)
	}
}

func TestActivateResolvedStreakCarryoverUsesRecentOfflineTime(t *testing.T) {
	now := time.Now()
	offlineAt := now.Add(-20 * time.Second)
	streamer := &domain.Streamer{
		OfflineAt: offlineAt,
	}

	ActivateCarryover(streamer, now)

	want := offlineAt.Add(CarryoverWindow)
	if !streamer.ResolvedStreakCarryoverUntil.Equal(want) {
		t.Fatalf("carryover expiry got %s want %s", streamer.ResolvedStreakCarryoverUntil, want)
	}
}
