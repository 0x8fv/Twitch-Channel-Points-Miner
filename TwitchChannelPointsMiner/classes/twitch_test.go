package classes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestTwitch(t *testing.T, responder func(operation string) (*http.Response, error)) *Twitch {
	t.Helper()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			_ = req.Body.Close()
			body := string(bodyBytes)

			var operation string
			switch {
			case strings.Contains(body, `"operationName":"VideoPlayerStreamInfoOverlayChannel"`):
				operation = "VideoPlayerStreamInfoOverlayChannel"
			case strings.Contains(body, `"operationName":"WithIsStreamLiveQuery"`):
				operation = "WithIsStreamLiveQuery"
			case strings.Contains(body, `"operationName":"RewardList"`):
				operation = "RewardList"
			default:
				t.Fatalf("unexpected request body: %s", body)
			}

			return responder(operation)
		}),
	}

	return &Twitch{
		userAgent:      "ua",
		deviceID:       "device",
		clientSession:  "session",
		clientVersion:  constants.ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin: &TwitchLogin{
			Token:  "token",
			userID: "user-id",
		},
		client: client,
	}
}

func newTestStreamer(watchStreak bool) *entities.Streamer {
	return &entities.Streamer{
		Username:  "streamer",
		ChannelID: "123456",
		Settings: entities.StreamerSettings{
			WatchStreak: watchStreak,
		},
		Stream: entities.NewStream(),
	}
}

func TestGetUserByIDReturnsCurrentLogin(t *testing.T) {
	twitch := &Twitch{
		userAgent: "ua",
		twitchLogin: &TwitchLogin{
			Token: "token",
		},
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("method got %s want GET", req.Method)
				}
				if req.URL.String() != "https://api.twitch.tv/helix/users?id=123456" {
					t.Fatalf("url got %s", req.URL.String())
				}
				if got := req.Header.Get("Authorization"); got != "Bearer token" {
					t.Fatalf("Authorization got %q", got)
				}
				if got := req.Header.Get("Client-Id"); got != constants.ClientID {
					t.Fatalf("Client-Id got %q", got)
				}
				return jsonResponse(http.StatusOK, `{"data":[{"id":"123456","login":"newlogin","display_name":"NewLogin"}]}`), nil
			}),
		},
	}

	user, err := twitch.GetUserByID("123456")
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if user.ID != "123456" || user.Login != "newlogin" || user.DisplayName != "NewLogin" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestGetUserByIDReturnsNotFoundForEmptyData(t *testing.T) {
	twitch := &Twitch{
		userAgent: "ua",
		twitchLogin: &TwitchLogin{
			Token: "token",
		},
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data":[]}`), nil
			}),
		},
	}

	_, err := twitch.GetUserByID("123456")
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestLoadChannelPointsContextWrapsMissingChannel(t *testing.T) {
	twitch := &Twitch{
		clientVersion:  constants.ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin:    &TwitchLogin{Token: "token"},
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data":{"community":{"channel":null}}}`), nil
			}),
		},
	}
	streamer := &entities.Streamer{Username: "oldlogin", ChannelID: "123456"}

	_, err := twitch.LoadChannelPointsContext(streamer)
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("expected ErrChannelNotFound, got %v", err)
	}
}

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
