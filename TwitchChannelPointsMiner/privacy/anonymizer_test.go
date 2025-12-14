package privacy

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestAnonymizer_NameStable(t *testing.T) {
	a := New(true)
	if got := a.Name(""); got != "" {
		t.Fatalf("expected empty input to stay empty, got %q", got)
	}
	first := a.Name("pewdiepie")
	second := a.Name("pewdiepie")
	if first == "" || second == "" {
		t.Fatalf("expected non-empty alias, got %q / %q", first, second)
	}
	if first != second {
		t.Fatalf("expected stable alias, got %q then %q", first, second)
	}
	if other := a.Name("ohnepixel"); other == "" || other == first {
		t.Fatalf("expected different alias for different name, got %q (other=%q)", first, other)
	}
}

func TestAnonymizer_PseudoChannelPointsFollowsDeltas(t *testing.T) {
	a := New(true)
	a.initialPointsMin = 500
	a.initialPointsMax = 500

	s := &entities.Streamer{Username: "pewdiepie", ChannelID: "123"}
	s.ChannelPoints = 1000
	first := a.PseudoChannelPoints(s)
	if first != 500 {
		t.Fatalf("expected deterministic initial pseudo points 500, got %d", first)
	}

	// No change should keep pseudo constant.
	s.ChannelPoints = 1000
	if got := a.PseudoChannelPoints(s); got != 500 {
		t.Fatalf("expected pseudo points unchanged, got %d", got)
	}

	// Delta should apply to pseudo.
	s.ChannelPoints = 1010
	if got := a.PseudoChannelPoints(s); got != 510 {
		t.Fatalf("expected pseudo points +10, got %d", got)
	}

	// Negative delta should apply too (e.g., betting).
	s.ChannelPoints = 1007
	if got := a.PseudoChannelPoints(s); got != 507 {
		t.Fatalf("expected pseudo points -3, got %d", got)
	}
}

func TestAnonymizer_DisabledPassthrough(t *testing.T) {
	a := New(false)
	if got := a.Name("pewdiepie"); got != "pewdiepie" {
		t.Fatalf("expected passthrough name when disabled, got %q", got)
	}
	s := &entities.Streamer{Username: "pewdiepie", ChannelID: "123", ChannelPoints: 4242}
	if got := a.PseudoChannelPoints(s); got != 4242 {
		t.Fatalf("expected passthrough points when disabled, got %d", got)
	}
}
