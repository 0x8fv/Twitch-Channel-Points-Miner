package chat

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestShouldJoin(t *testing.T) {
	tests := []struct {
		mode   domain.IRCMode
		online bool
		want   bool
	}{
		{mode: domain.IRCModeAlways, online: true, want: true},
		{mode: domain.IRCModeAlways, online: false, want: true},
		{mode: domain.IRCModeNever, online: true, want: false},
		{mode: domain.IRCModeNever, online: false, want: false},
		{mode: domain.IRCModeOnline, online: true, want: true},
		{mode: domain.IRCModeOnline, online: false, want: false},
		{mode: domain.IRCModeOffline, online: true, want: false},
		{mode: domain.IRCModeOffline, online: false, want: true},
		{mode: domain.IRCMode(""), online: true, want: true}, // ? default fallback
		{mode: domain.IRCMode("unknown"), online: false, want: false},
	}
	for _, tt := range tests {
		if got := ShouldJoin(tt.mode, tt.online); got != tt.want {
			t.Fatalf("mode=%s online=%t got %t want %t", tt.mode, tt.online, got, tt.want)
		}
	}
}
