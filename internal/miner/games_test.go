package miner

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestGameInfoRespectsShowFlag(t *testing.T) {
	streamer := &domain.Streamer{
		Stream: &domain.Stream{
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
