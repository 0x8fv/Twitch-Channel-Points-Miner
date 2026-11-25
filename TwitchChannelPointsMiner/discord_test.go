package twitchchannelpointsminer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

func TestDiscordWebhookSendsEmbed(t *testing.T) {
	var bodies []discordPayload
	var types []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		types = append(types, r.Header.Get("Content-Type"))
		raw, _ := io.ReadAll(r.Body)
		var payload discordPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("decode: %v", err)
		}
		bodies = append(bodies, payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := NewDiscordWebhook(DiscordSettings{WebhookAPI: srv.URL})
	if d == nil {
		t.Fatal("webhook should be enabled")
	}
	line := "🥳 Alice (1.2k points) is Online! | Playing: Valorant | streak length 2 | expires at 2026-11-25 15:04:05 +0100 CET"
	d.Send(line, constants.EventStreamerOnline)
	d.Send("tick", constants.EventGainForWatch) // quiet, not selected
	d.Send("ignored", "")

	if len(bodies) != 1 {
		t.Fatalf("got %d requests, want 1", len(bodies))
	}
	if types[0] != "application/json" {
		t.Fatalf("content type %q", types[0])
	}
	embed := bodies[0].Embeds[0]
	if embed.Title != "Online" || embed.Description != "Alice is online with 1.2k points, playing Valorant. The watch streak is 2." || embed.Color != 0x0A7A2F {
		t.Fatalf("unexpected embed %+v", embed)
	}
	if bodies[0].Username != discordUsername {
		t.Fatalf("username %q", bodies[0].Username)
	}

	named := NewDiscordWebhook(DiscordSettings{WebhookAPI: srv.URL, Events: []string{"GAIN_FOR_WATCH"}})
	named.Send("tick", constants.EventGainForWatch)
	if len(bodies) != 2 || !strings.HasPrefix(bodies[1].Embeds[0].Title, "Watch") {
		t.Fatalf("explicit watch event should be delivered, got %+v", bodies)
	}
}

func TestDiscordWebhookDisabledWithoutURL(t *testing.T) {
	if NewDiscordWebhook(DiscordSettings{}) != nil {
		t.Fatal("webhook must be nil without a url")
	}
	var d *DiscordWebhook
	d.Send("ignored", constants.EventStartup)
}
