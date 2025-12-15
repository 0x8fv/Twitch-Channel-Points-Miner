package entities

import (
	"testing"
	"time"
)

func TestBetSettingsDefault(t *testing.T) {
	b := BetSettings{}
	b.Default()
	if b.Strategy == "" || b.Percentage == nil || b.PercentageGap == nil || b.MaxPoints == nil || b.MinimumPoints == nil || b.StealthMode == nil || b.DelayMode == "" || b.Delay == nil {
		t.Fatalf("defaults not applied: %#v", b)
	}
}

func TestStreamerSettingsDefault(t *testing.T) {
	s := StreamerSettings{}
	s.Default()
	if s.Bet.Strategy == "" || s.Bet.Delay == nil {
		t.Fatalf("bet defaults not propagated: %#v", s)
	}
	if s.IRCMode == "" || s.IRCMode != IRCModeOnline {
		t.Fatalf("irc mode default not applied: %s", s.IRCMode)
	}
}

func TestStreamerMultipliers(t *testing.T) {
	streamer := &Streamer{
		ActiveMultipliers: []ActiveMultiplier{
			{Factor: 1.5},
			{Factor: 2},
		},
	}
	if !streamer.HasActiveMultipliers() {
		t.Fatalf("expected active multipliers")
	}
	if total := streamer.TotalMultiplier(); total != 3.5 {
		t.Fatalf("total multiplier mismatch got %f", total)
	}
}

func TestPredictionWindowSeconds(t *testing.T) {
	delay := 2.0
	settings := StreamerSettings{
		Bet: BetSettings{
			Delay:     &delay,
			DelayMode: DelayModeFromStart,
		},
	}
	streamer := &Streamer{Settings: settings}
	if got := streamer.PredictionWindowSeconds(5); got != 2 {
		t.Fatalf("from start delay got %f", got)
	}

	settings.Bet.DelayMode = DelayModeFromEnd
	streamer.Settings = settings
	if got := streamer.PredictionWindowSeconds(5); got != 3 {
		t.Fatalf("from end delay got %f", got)
	}

	settings.Bet.DelayMode = DelayModePercentage
	delay = 0.5
	settings.Bet.Delay = &delay
	streamer.Settings = settings
	if got := streamer.PredictionWindowSeconds(10); got != 5 {
		t.Fatalf("percentage delay got %f", got)
	}
}

func TestStreamUpElapsed(t *testing.T) {
	stream := &Stream{}
	if !stream.StreamUpElapsed() {
		t.Fatalf("zero time should be elapsed")
	}
	stream.StreamUpAt = time.Now()
	if stream.StreamUpElapsed() {
		t.Fatalf("fresh stream should not be elapsed")
	}
}
