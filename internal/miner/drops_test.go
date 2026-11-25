package miner

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestShouldClaimDrops(t *testing.T) {
	tests := []struct {
		name      string
		global    bool
		streamers []*domain.Streamer
		want      bool
	}{
		{
			name:   "enabled globally",
			global: true,
			want:   true,
		},
		{
			name:   "enabled by streamer override",
			global: false,
			streamers: []*domain.Streamer{
				{Settings: domain.StreamerSettings{ClaimDrops: false}},
				{Settings: domain.StreamerSettings{ClaimDrops: true}},
			},
			want: true,
		},
		{
			name:   "disabled globally and per streamer",
			global: false,
			streamers: []*domain.Streamer{
				{Settings: domain.StreamerSettings{ClaimDrops: false}},
				nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Miner{
				StreamerSettings: domain.StreamerSettings{
					ClaimDrops: tt.global,
				},
			}
			if got := m.shouldClaimDrops(tt.streamers); got != tt.want {
				t.Fatalf("shouldClaimDrops() = %t, want %t", got, tt.want)
			}
		})
	}
}
