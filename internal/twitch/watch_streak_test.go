package twitch

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExtractWatchStreakAchievementAt(t *testing.T) {
	resp := &gqlRewardListResponse{}
	if err := json.Unmarshal(
		[]byte(`{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`),
		resp,
	); err != nil {
		t.Fatalf("unmarshal reward list response: %v", err)
	}

	achievementAt := extractWatchStreakAchievementAt(resp)
	if achievementAt.IsZero() {
		t.Fatalf("expected achievement timestamp to be extracted")
	}
	if !achievementAt.Equal(time.Date(2026, time.March, 1, 10, 6, 0, 0, time.UTC)) {
		t.Fatalf("unexpected achievement timestamp: %s", achievementAt)
	}
}

func TestCurrentStreamHasCompletedWatchStreak(t *testing.T) {
	createdAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	achievementAt := createdAt.Add(6 * time.Minute)
	if !currentStreamHasCompletedWatchStreak(createdAt, achievementAt) {
		t.Fatalf("expected later achievement timestamp to complete streak")
	}
	if currentStreamHasCompletedWatchStreak(createdAt, createdAt.Add(-time.Minute)) {
		t.Fatalf("older achievement timestamp should not complete streak")
	}
	if currentStreamHasCompletedWatchStreak(time.Time{}, achievementAt) {
		t.Fatalf("missing stream start timestamp should not complete streak")
	}
}
