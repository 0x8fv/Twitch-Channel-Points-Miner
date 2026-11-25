package miner

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/twitch"
)

func TestFormatDropProgressBar(t *testing.T) {
	if got := formatDropProgressBar(63, 180); got != "[#######-------------]" {
		t.Fatalf("progress bar got %q", got)
	}
	if got := formatDropProgressBar(0, 0); got != "[--------------------]" {
		t.Fatalf("empty progress bar got %q", got)
	}
	if got := formatDropProgressBar(250, 180); got != "[####################]" {
		t.Fatalf("complete progress bar got %q", got)
	}
}

func TestLogDropStatusesHonorsProgressFlag(t *testing.T) {
	var buf bytes.Buffer
	m := &Miner{
		logger: &Logger{base: log.New(&buf, "", 0)},
	}
	statuses := []twitch.DropStatus{
		{
			RewardName:    "Summer Token Store #12",
			CampaignName:  "World of Tanks",
			GameName:      "World of Tanks",
			ChannelName:   "example_streamer",
			CurrentValue:  63,
			RequiredValue: 180,
		},
	}

	m.logDropStatuses(statuses)
	if buf.Len() != 0 {
		t.Fatalf("progress logs should be disabled by default, got %q", buf.String())
	}

	m.showDropsProgress = true
	m.logDropStatuses(statuses)
	got := buf.String()
	want := "DROP_PROGRESS [#######-------------] 35% | World of Tanks / Summer Token Store #12 / example_streamer / 63m of 180m / remaining 117m / IN_PROGRESS"
	if !strings.Contains(got, want) {
		t.Fatalf("progress log got %q, want to contain %q", got, want)
	}
}

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
