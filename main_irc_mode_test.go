package main

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func TestParseIRCMode(t *testing.T) {
	fallback := entities.IRCModeOnline
	tests := []struct {
		name string
		in   string
		want entities.IRCMode
	}{
		{name: "always exact", in: "ALWAYS", want: entities.IRCModeAlways},
		{name: "always lower", in: "always", want: entities.IRCModeAlways},
		{name: "never padded", in: "  never ", want: entities.IRCModeNever},
		{name: "offline", in: "offline", want: entities.IRCModeOffline},
		{name: "online", in: "online", want: entities.IRCModeOnline},
		{name: "empty", in: "", want: fallback},
		{name: "invalid", in: "nope", want: fallback},
	}
	for _, tt := range tests {
		if got := parseChatPresence(tt.in, fallback); got != tt.want {
			t.Fatalf("%s: parseChatPresence(%q)=%s want %s", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestMergeStreamerSettingsIRCMode(t *testing.T) {
	base := entities.StreamerSettings{IRCMode: entities.IRCModeNever}

	// ? Valid override should replace base
	overrideVal := "online"
	out := mergeStreamerSettings(base, streamerSettingsConfig{IRCMode: &overrideVal})
	if out.IRCMode != entities.IRCModeOnline {
		t.Fatalf("expected override to set IRCMode to ONLINE, got %s", out.IRCMode)
	}

	// ? Invalid override should keep base
	invalid := "maybe"
	out = mergeStreamerSettings(base, streamerSettingsConfig{IRCMode: &invalid})
	if out.IRCMode != base.IRCMode {
		t.Fatalf("invalid override should keep base IRCMode %s, got %s", base.IRCMode, out.IRCMode)
	}

	// ? Nil override should keep base
	out = mergeStreamerSettings(base, streamerSettingsConfig{})
	if out.IRCMode != base.IRCMode {
		t.Fatalf("nil override should keep base IRCMode %s, got %s", base.IRCMode, out.IRCMode)
	}
}
