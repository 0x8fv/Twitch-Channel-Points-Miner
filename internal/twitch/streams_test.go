package twitch

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestParseRFC3339Timestamp(t *testing.T) {
	parsed := parseRFC3339Timestamp("2026-03-01T10:00:00Z")
	if parsed.IsZero() {
		t.Fatalf("expected valid timestamp to parse")
	}
	if !parsed.Equal(time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected parsed timestamp: %s", parsed)
	}
	if got := parseRFC3339Timestamp("not-a-time"); !got.IsZero() {
		t.Fatalf("invalid timestamp should return zero time")
	}
}

func TestCheckStreamerOnlineSkipsRewardListAndUsesFallbackCreatedAt(t *testing.T) {
	rewardListCalls := 0
	liveQueryCalls := 0
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{"displayName":"Game"}}}}}`), nil
		case "WithIsStreamLiveQuery":
			liveQueryCalls++
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"createdAt":"2026-03-01T10:00:00Z"}}}}`), nil
		case "RewardList":
			rewardListCalls++
			return jsonResponse(http.StatusOK, `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(true)

	online, err := twitch.CheckStreamerOnline(streamer)
	if err != nil {
		t.Fatalf("CheckStreamerOnline returned error: %v", err)
	}
	if !online {
		t.Fatalf("expected streamer to be online")
	}
	if rewardListCalls != 0 {
		t.Fatalf("CheckStreamerOnline should not query RewardList")
	}
	if liveQueryCalls != 1 {
		t.Fatalf("WithIsStreamLiveQuery call count got %d want 1", liveQueryCalls)
	}
	expected := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	if !streamer.Stream.CreatedAt.Equal(expected) {
		t.Fatalf("createdAt got %s want %s", streamer.Stream.CreatedAt, expected)
	}
	if got := streamer.Stream.GameName(); got != "Game" {
		t.Fatalf("game name got %q want %q", got, "Game")
	}
}

func TestUpdateStreamMarksWatchStreakCompleteFromRewardListMilestone(t *testing.T) {
	rewardListCalls := 0
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","createdAt":"2026-03-01T10:00:00Z","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{}}}}}`), nil
		case "RewardList":
			rewardListCalls++
			return jsonResponse(http.StatusOK, `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(true)

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream returned error: %v", err)
	}
	if rewardListCalls != 1 {
		t.Fatalf("RewardList call count got %d want 1", rewardListCalls)
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("milestone should mark watch streak as complete")
	}
	if !streamer.CompletedWatchStreak {
		t.Fatalf("milestone should preserve the actual completion signal")
	}
	if streamer.Stream.CreatedAt.IsZero() {
		t.Fatalf("expected stream createdAt to be captured")
	}
}

func TestUpdateStreamPreservesActualCompletionWhenWatchStreakAlreadyMasked(t *testing.T) {
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","createdAt":"2026-03-01T10:00:00Z","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{}}}}}`), nil
		case "RewardList":
			return jsonResponse(http.StatusOK, `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(true)
	streamer.Stream.WatchStreakMissing = false

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream returned error: %v", err)
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("masked streak state should remain resolved")
	}
	if !streamer.CompletedWatchStreak {
		t.Fatalf("masked streak state should still record the actual completion")
	}
}

func TestUpdateStreamUsesLiveFallbackCreatedAtForMilestoneComparison(t *testing.T) {
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{}}}}}`), nil
		case "WithIsStreamLiveQuery":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"createdAt":"2026-03-01T10:00:00Z"}}}}`), nil
		case "RewardList":
			return jsonResponse(http.StatusOK, `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(true)

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream returned error: %v", err)
	}
	if streamer.Stream.WatchStreakMissing {
		t.Fatalf("fallback createdAt should allow milestone inference")
	}
	expected := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	if !streamer.Stream.CreatedAt.Equal(expected) {
		t.Fatalf("createdAt got %s want %s", streamer.Stream.CreatedAt, expected)
	}
}

func TestUpdateStreamIgnoresRewardListFailure(t *testing.T) {
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","createdAt":"2026-03-01T10:00:00Z","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{}}}}}`), nil
		case "RewardList":
			return nil, errors.New("reward list failed")
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(true)

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream should ignore RewardList errors, got %v", err)
	}
	if !streamer.Stream.WatchStreakMissing {
		t.Fatalf("reward list failure should keep streak pending")
	}
}

func TestUpdateStreamSkipsRewardListWhenWatchStreakDisabled(t *testing.T) {
	rewardListCalls := 0
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","createdAt":"2026-03-01T10:00:00Z","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{}}}}}`), nil
		case "RewardList":
			rewardListCalls++
			return jsonResponse(http.StatusOK, `{"data":{"channel":{"self":{"watchStreakMilestone":{"watchStreakMilestone":{"achievementTimestamp":"2026-03-01T10:06:00Z"}}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(false)

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream returned error: %v", err)
	}
	if rewardListCalls != 0 {
		t.Fatalf("RewardList should not be queried when watch streak is disabled")
	}
	if !streamer.Stream.WatchStreakMissing {
		t.Fatalf("stream should remain pending without streak inference")
	}
}

func TestUpdateStreamIncludesGameMetadataWhenClaimDropsDisabled(t *testing.T) {
	twitch := newTestTwitch(t, func(operation string) (*http.Response, error) {
		switch operation {
		case "VideoPlayerStreamInfoOverlayChannel":
			return jsonResponse(http.StatusOK, `{"data":{"user":{"stream":{"id":"broadcast-1","createdAt":"2026-03-01T10:00:00Z","viewersCount":42,"tags":[]},"broadcastSettings":{"title":"title","game":{"id":"game-1","name":"Game","displayName":"Game"}}}}}`), nil
		default:
			t.Fatalf("unexpected operation: %s", operation)
			return nil, nil
		}
	})
	streamer := newTestStreamer(false)
	streamer.Settings.ClaimDrops = false

	if err := twitch.UpdateStream(streamer); err != nil {
		t.Fatalf("UpdateStream returned error: %v", err)
	}
	if len(streamer.Stream.Payload) != 1 {
		t.Fatalf("payload count got %d want 1", len(streamer.Stream.Payload))
	}
	props, ok := streamer.Stream.Payload[0]["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload properties missing")
	}
	if props["game"] != "Game" {
		t.Fatalf("game got %#v want Game in payload %#v", props["game"], props)
	}
	if props["game_id"] != "game-1" {
		t.Fatalf("game_id got %#v want game-1 in payload %#v", props["game_id"], props)
	}
}
