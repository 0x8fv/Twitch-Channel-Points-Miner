package twitch

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
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
		clientVersion:  ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin: &TwitchLogin{
			Token:  "token",
			userID: "user-id",
		},
		client: client,
	}
}

func newTestStreamer(watchStreak bool) *domain.Streamer {
	return &domain.Streamer{
		Username:  "streamer",
		ChannelID: "123456",
		Settings: domain.StreamerSettings{
			WatchStreak: watchStreak,
		},
		Stream: domain.NewStream(),
	}
}

type stubPubSubLogger struct {
	printfCalls int
	errorCalls  int
	emojiCalls  int
	debugCalls  int
}

func (s *stubPubSubLogger) Printf(string, ...interface{}) {
	s.printfCalls++
}

func (s *stubPubSubLogger) Errorf(string, ...interface{}) {
	s.errorCalls++
}

func (s *stubPubSubLogger) EmojiPrintf(string, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubPubSubLogger) Eventf(events.Event, string, ...interface{}) {
	s.printfCalls++
}

func (s *stubPubSubLogger) EmojiEventf(string, events.Event, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubPubSubLogger) ErrorEventf(events.Event, string, ...interface{}) {
	s.errorCalls++
}

func (s *stubPubSubLogger) Debugf(string, ...interface{}) {
	s.debugCalls++
}

func (s *stubPubSubLogger) DebugEnabled() bool {
	return false
}
