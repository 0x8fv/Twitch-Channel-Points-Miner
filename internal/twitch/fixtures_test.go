package twitch

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/twitchapi"
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

func newTestTwitch(t *testing.T, responder func(operation string) (*http.Response, error)) *Client {
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

	return &Client{
		userAgent:      "ua",
		deviceID:       "device",
		clientSession:  "session",
		clientVersion:  twitchapi.ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin:    fakeSession{token: "token", userID: "user-id"},
		client:         client,
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

type stubLogger struct {
	printfCalls int
	errorCalls  int
	emojiCalls  int
	debugCalls  int
}

func (s *stubLogger) Printf(string, ...interface{}) {
	s.printfCalls++
}

func (s *stubLogger) Errorf(string, ...interface{}) {
	s.errorCalls++
}

func (s *stubLogger) EmojiPrintf(string, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubLogger) Eventf(events.Event, string, ...interface{}) {
	s.printfCalls++
}

func (s *stubLogger) EmojiEventf(string, events.Event, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubLogger) ErrorEventf(events.Event, string, ...interface{}) {
	s.errorCalls++
}

func (s *stubLogger) Debugf(string, ...interface{}) {
	s.debugCalls++
}

func (s *stubLogger) DebugEnabled() bool {
	return false
}

// fakeSession stands in for a logged-in account.
type fakeSession struct {
	token  string
	userID string
}

func (s fakeSession) Login(string) error { return nil }
func (s fakeSession) AuthToken() string  { return s.token }
func (s fakeSession) UserID() string     { return s.userID }
