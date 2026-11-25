package twitch

import (
	"errors"
	"net/http"
	"testing"

	"TwitchChannelPointsMiner/internal/auth"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func TestGetUserByIDReturnsCurrentLogin(t *testing.T) {
	twitch := &Client{
		userAgent: "ua",
		twitchLogin: &auth.Login{
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
				if got := req.Header.Get("Client-Id"); got != twitchapi.ClientID {
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
	twitch := &Client{
		userAgent: "ua",
		twitchLogin: &auth.Login{
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
