package twitchchannelpointsminer

import (
	"net/http"
	"net/url"
	"strings"
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

func (d *DiscordWebhook) Send(message string, event constants.Event) {
	if d == nil {
		return
	}
	if len(d.events) > 0 {
		if _, ok := d.events[event]; !ok {
			return
		}
	} else if event == "" {
		return
	}
	payload := url.Values{
		"content":    []string{message},
		"username":   []string{discordUsername},
		"avatar_url": []string{discordAvatarURL},
	}
	req, err := http.NewRequest(http.MethodPost, d.webhookAPI, strings.NewReader(payload.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
