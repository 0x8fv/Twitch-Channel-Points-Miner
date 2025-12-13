package twitchchannelpointsminer

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestShouldJoinChat(t *testing.T) {
	tests := []struct {
		mode   entities.IRCMode
		online bool
		want   bool
	}{
		{mode: entities.IRCModeAlways, online: true, want: true},
		{mode: entities.IRCModeAlways, online: false, want: true},
		{mode: entities.IRCModeNever, online: true, want: false},
		{mode: entities.IRCModeNever, online: false, want: false},
		{mode: entities.IRCModeOnline, online: true, want: true},
		{mode: entities.IRCModeOnline, online: false, want: false},
		{mode: entities.IRCModeOffline, online: true, want: false},
		{mode: entities.IRCModeOffline, online: false, want: true},
		{mode: entities.IRCMode(""), online: true, want: true}, // ? default fallback
		{mode: entities.IRCMode("unknown"), online: false, want: false},
	}
	for _, tt := range tests {
		if got := shouldJoinChat(tt.mode, tt.online); got != tt.want {
			t.Fatalf("mode=%s online=%t got %t want %t", tt.mode, tt.online, got, tt.want)
		}
	}
}
