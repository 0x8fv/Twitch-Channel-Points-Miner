package twitch

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/auth"
	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func TestLoadChannelPointsContextWrapsMissingChannel(t *testing.T) {
	twitch := &Client{
		clientVersion:  twitchapi.ClientVersion,
		versionTTL:     time.Hour,
		versionFetched: time.Now(),
		twitchLogin:    &auth.Login{Token: "token"},
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data":{"community":{"channel":null}}}`), nil
			}),
		},
	}
	streamer := &domain.Streamer{Username: "oldlogin", ChannelID: "123456"}

	_, err := twitch.LoadChannelPointsContext(streamer)
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("expected ErrChannelNotFound, got %v", err)
	}
}
