package notify

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

const (
	discordUsername  = "Twitch Channel Points Miner"
	discordAvatarURL = "https://raw.githubusercontent.com/0x8fv/Twitch-Channel-Points-Miner/main/assets/gopher.png"
)

type DiscordSettings struct {
	WebhookAPI string   `json:"webhook_api"`
	Events     []string `json:"events"`
}

type DiscordWebhook struct {
	webhookAPI string
	events     map[constants.Event]struct{}
	client     *http.Client
	warnOnce   sync.Once
}

func NewDiscordWebhook(settings DiscordSettings) *DiscordWebhook {
	webhookAPI := strings.TrimSpace(settings.WebhookAPI)
	if webhookAPI == "" {
		return nil
	}
	events := make(map[constants.Event]struct{})
	for _, raw := range settings.Events {
		event := constants.NormalizeEventName(raw)
		if event == "" {
			continue
		}
		events[event] = struct{}{}
	}
	return &DiscordWebhook{
		webhookAPI: webhookAPI,
		events:     events,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Color       int    `json:"color"`
}

type discordPayload struct {
	Username  string         `json:"username"`
	AvatarURL string         `json:"avatar_url"`
	Embeds    []discordEmbed `json:"embeds"`
}

// Send posts a short embed if the event is selected. An empty events list sends
// every event except the noisy ones, matching Telegram. Failures never interrupt
// the miner; the webhook URL (it contains a secret) is never logged.
func (d *DiscordWebhook) Send(message string, event constants.Event) {
	if d == nil || !wantsEvent(d.events, event) {
		return
	}
	body := Sentence(message)
	title := eventGlyphPrefix(event) + eventTitle(event)
	if strings.EqualFold(body, title) {
		body = ""
	}
	if r := []rune(title); len(r) > 256 {
		title = string(r[:256])
	}
	if r := []rune(body); len(r) > 4096 {
		body = string(r[:4095]) + "…"
	}
	payload := discordPayload{
		Username:  discordUsername,
		AvatarURL: discordAvatarURL,
		Embeds: []discordEmbed{{
			Title:       title,
			Description: body,
			Color:       eventColor(event),
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, d.webhookAPI, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		d.warn("request failed (network error or timeout)")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 180))
	reason := strings.TrimSpace(string(snippet))
	if reason == "" {
		reason = resp.Status
	}
	d.warn(reason)
}

func (d *DiscordWebhook) warn(reason string) {
	d.warnOnce.Do(func() {
		log.Printf("discord: notification not delivered: %s (further failures are not logged)", reason)
	})
}
