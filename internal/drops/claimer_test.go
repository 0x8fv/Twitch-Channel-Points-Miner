package drops

import (
	"bytes"
	"strings"
	"testing"

	"TwitchChannelPointsMiner/internal/logging"
	"TwitchChannelPointsMiner/internal/twitch"
)

func TestLogStatusesHonorsProgressFlag(t *testing.T) {
	var buf bytes.Buffer
	c := NewClaimer(nil, logging.NewLoggerTo(&buf, logging.LoggerSettings{}), false)
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

	c.logStatuses(statuses)
	if buf.Len() != 0 {
		t.Fatalf("progress logs should be disabled by default, got %q", buf.String())
	}

	c.showProgress = true
	c.logStatuses(statuses)
	got := buf.String()
	want := "DROP_PROGRESS [#######-------------] 35% | World of Tanks / Summer Token Store #12 / example_streamer / 63m of 180m / remaining 117m / IN_PROGRESS"
	if !strings.Contains(got, want) {
		t.Fatalf("progress log got %q, want to contain %q", got, want)
	}
}
