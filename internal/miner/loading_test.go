package miner

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/logging"
)

func TestLoadSummaryIsOneLine(t *testing.T) {
	m := &Miner{}
	streamers := []*domain.Streamer{
		{Username: "alice", IsOnline: true},
		{Username: "bob"},
		{Username: "carol", IsOnline: true},
	}
	got := m.loadSummary(streamers, 5200*time.Millisecond)
	if strings.Contains(got, "\n") {
		t.Fatalf("summary should be one line, got %q", got)
	}
	for _, want := range []string{"3 streamers loaded in 5.2 seconds", "2 online (", "lice", "arol", ", 1 offline"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "ob,") || strings.Contains(got, "ob)") {
		t.Fatalf("offline streamers should not be named, got %q", got)
	}

	one := m.loadSummary([]*domain.Streamer{{Username: "bob"}}, time.Second)
	if !strings.Contains(one, "1 streamer loaded") || !strings.Contains(one, "0 online, 1 offline") {
		t.Fatalf("unexpected summary for one offline streamer: %q", one)
	}
}

func TestLoadSummaryCapsTheNames(t *testing.T) {
	m := &Miner{}
	var streamers []*domain.Streamer
	for i := 0; i < maxNamesInSummary+3; i++ {
		streamers = append(streamers, &domain.Streamer{Username: "s" + string(rune('a'+i)), IsOnline: true})
	}
	got := m.loadSummary(streamers, time.Second)
	if !strings.Contains(got, "and 3 more") {
		t.Fatalf("long lists should be cut short, got %q", got)
	}
}

func TestPresenceIsNotAnnouncedWhileStartingUp(t *testing.T) {
	var buf bytes.Buffer
	m := &Miner{logger: logging.NewLoggerTo(&buf, logging.LoggerSettings{})}

	m.startingUp.Store(true)
	m.setPresence(&domain.Streamer{Username: "alice", Stream: domain.NewStream()}, false, "poll")
	if buf.Len() != 0 {
		t.Fatalf("nothing should be logged per streamer while starting up, got %q", buf.String())
	}

	m.startingUp.Store(false)
	m.setPresence(&domain.Streamer{Username: "bob", Stream: domain.NewStream()}, false, "poll")
	if !strings.Contains(buf.String(), "Offline") {
		t.Fatalf("presence should be announced once startup is over, got %q", buf.String())
	}
}
