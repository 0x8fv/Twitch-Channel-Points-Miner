package miner

import (
	"errors"
	"fmt"
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitch"
)

func TestStreamerInitialPointsKeyUsesChannelIDWhenAvailable(t *testing.T) {
	streamer := &domain.Streamer{Username: "oldlogin", ChannelID: "123456"}

	if got := streamerInitialPointsKey(streamer); got != "123456" {
		t.Fatalf("key got %q want channel id", got)
	}

	streamer.ChannelID = ""
	if got := streamerInitialPointsKey(streamer); got != "oldlogin" {
		t.Fatalf("fallback key got %q want username", got)
	}
}

func TestApplyStreamerLoginChangeMigratesMutableLoginState(t *testing.T) {
	streamer := &domain.Streamer{
		Username:      "oldlogin",
		ChannelID:     "123456",
		ChannelPoints: 250,
		StreamerURL:   "https://www.twitch.tv/oldlogin",
		Stream:        domain.NewStream(),
	}
	streamer.Stream.SpadeURL = "https://spade.example/old"

	m := &Miner{
		initialPoints: map[string]int{
			"123456":   100,
			"oldlogin": 100,
		},
	}

	changed := m.applyStreamerLoginChange(streamer, "newlogin")
	if !changed {
		t.Fatalf("expected login change")
	}
	if streamer.Username != "newlogin" {
		t.Fatalf("username got %q", streamer.Username)
	}
	if streamer.StreamerURL != "https://www.twitch.tv/newlogin" {
		t.Fatalf("streamer url got %q", streamer.StreamerURL)
	}
	if streamer.Stream.SpadeURL != "" {
		t.Fatalf("spade url should be cleared after login change")
	}
	if got := m.initialPoints["123456"]; got != 100 {
		t.Fatalf("stable initial points got %d", got)
	}
}

func TestShouldRepairStreamerIdentityOnlyForChannelNotFound(t *testing.T) {
	if !shouldRepairStreamerIdentity(fmt.Errorf("wrapped: %w", twitch.ErrChannelNotFound)) {
		t.Fatalf("ErrChannelNotFound should trigger identity repair")
	}
	if shouldRepairStreamerIdentity(twitch.ErrStreamerOffline) {
		t.Fatalf("ErrStreamerOffline should not trigger identity repair")
	}
	if shouldRepairStreamerIdentity(errors.New("temporary network error")) {
		t.Fatalf("generic errors should not trigger identity repair")
	}
}
