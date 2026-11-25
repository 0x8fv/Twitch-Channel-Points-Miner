package miner

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
	"TwitchChannelPointsMiner/internal/logging"
)

func TestNormalizeGameList(t *testing.T) {
	got := normalizeGameList([]string{" Valorant ", "Tom Clancy's Rainbow Six Siege X", "five night's at freddys", "valorant", "   "})
	want := []string{"valorant", "tom clancy's rainbow six siege x", "five night's at freddys"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestFilterExcludedTargets(t *testing.T) {
	m := &Miner{
		streamerExclusions: map[string]struct{}{
			"foo": {},
			"bar": {},
		},
	}

	got, excluded := m.filterExcludedTargets([]string{"Foo", "baz", "  bar  ", "", "qux"})
	want := []string{"baz", "qux"}

	if excluded != 2 {
		t.Fatalf("expected 2 excluded got %d", excluded)
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i], want[i])
		}
	}
}

func TestNewMinerDisableAtInNicknameSetting(t *testing.T) {
	streamerSettings := domain.StreamerSettings{}
	streamerSettings.Default()

	minr := NewMiner(
		"user",
		"",
		false,
		false,
		logging.LoggerSettings{},
		streamerSettings,
		nil,
		nil,
		nil,
		nil,
		nil,
		true,
		false,
		false,
		true,
		false,
	)

	if !minr.disableAtInNickname {
		t.Fatalf("expected disableAtInNickname to be enabled from constructor")
	}
}
