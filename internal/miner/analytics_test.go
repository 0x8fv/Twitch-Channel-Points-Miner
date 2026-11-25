package miner

import (
	"sync"
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/logging"
)

type fakeAnalyticsSink struct {
	mu     sync.Mutex
	points []string
	events []string
}

func (f *fakeAnalyticsSink) RecordPoint(streamer string, points, delta int, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.points = append(f.points, streamer+"|"+reason)
	_ = points
	_ = delta
}

func (f *fakeAnalyticsSink) RecordSnapshot(string, int, bool) {}

func (f *fakeAnalyticsSink) RecordEvent(event, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event+"|"+message)
}

func TestAnalyticsSinkReceivesPointsAndEvents(t *testing.T) {
	sink := &fakeAnalyticsSink{}
	SetAnalyticsSink(sink)
	defer SetAnalyticsSink(nil)

	m := &Miner{logger: logging.NewLogger(logging.LoggerSettings{}, "")}
	streamer := &domain.Streamer{Username: "alice", ChannelPoints: 1000}

	m.handlePointsUpdate(streamer, 0, "") // first load: baseline only
	streamer.ChannelPoints = 1450
	m.handlePointsUpdate(streamer, 1000, "WATCH_STREAK")
	m.logger.Eventf(events.EventStreamerOnline, "%sAlice%s is Online!", colorGreen, colorReset)
	m.logger.Printf("plain log lines are not events")

	if len(sink.points) != 2 || sink.points[0] != "alice|" || sink.points[1] != "alice|WATCH_STREAK" {
		t.Fatalf("unexpected points %v", sink.points)
	}
	// The watch-streak gain is itself logged as an event, followed by the explicit one.
	last := sink.events[len(sink.events)-1]
	if last != "STREAMER_ONLINE|Alice is online." {
		t.Fatalf("event should arrive as a short summary, got %q (all: %v)", last, sink.events)
	}
}
