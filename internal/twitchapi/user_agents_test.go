package twitchapi

import (
	"testing"
)

func TestGetUserAgent(t *testing.T) {
	ua := GetUserAgent("ignored")
	if ua == "" || ua != UserAgents["Android"]["TV"] {
		t.Fatalf("unexpected user agent: %q", ua)
	}
}
