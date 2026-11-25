package notify

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

// Telegram Bot API: https://core.telegram.org/bots/api#sendmessage
const (
	telegramAPIBase    = "https://api.telegram.org"
	telegramTextLimit  = 4096 // sendMessage accepts 1-4096 characters
	telegramNoPreviews = `{"is_disabled":true}`
)

type TelegramSettings struct {
	ChatID              string   `json:"chat_id"`
	Token               string   `json:"token"`
	Events              []string `json:"events"`
	DisableNotification bool     `json:"disable_notification"`
}

// TelegramNotifier sends selected log events to a Telegram chat through a bot.
type TelegramNotifier struct {
	endpoint            string
	chatID              string
	events              map[constants.Event]struct{}
	disableNotification bool
	client              *http.Client
	warnOnce            sync.Once
}

// NewTelegramNotifier returns nil unless both chat_id and token are set.
func NewTelegramNotifier(settings TelegramSettings) *TelegramNotifier {
	return newTelegramNotifier(settings, telegramAPIBase)
}

func newTelegramNotifier(settings TelegramSettings, apiBase string) *TelegramNotifier {
	chatID := strings.TrimSpace(settings.ChatID)
	token := strings.TrimSpace(settings.Token)
	if chatID == "" || token == "" {
		return nil
	}
	events := make(map[constants.Event]struct{})
	for _, raw := range settings.Events {
		if event := constants.NormalizeEventName(raw); event != "" {
			events[event] = struct{}{}
		}
	}
	return &TelegramNotifier{
		endpoint:            apiBase + "/bot" + token + "/sendMessage",
		chatID:              chatID,
		events:              events,
		disableNotification: settings.DisableNotification,
		client:              &http.Client{Timeout: 5 * time.Second},
	}
}

// Send posts a short HTML summary if the event is selected. An empty events
// list sends every event except the noisy ones (watch ticks, drop progress,
// bet-filter skips); "*" or "ALL" sends those too. Failures never interrupt the miner.
func (t *TelegramNotifier) Send(message string, event constants.Event) {
	if t == nil || !wantsEvent(t.events, event) {
		return
	}
	text := formatTelegram(event, message)
	if text == "" {
		return
	}
	payload := url.Values{
		"chat_id":              []string{t.chatID},
		"text":                 []string{text},
		"parse_mode":           []string{"HTML"},
		"disable_notification": []string{strconv.FormatBool(t.disableNotification)},
		"link_preview_options": []string{telegramNoPreviews},
	}
	resp, err := t.client.PostForm(t.endpoint, payload)
	if err != nil {
		// The error text contains the request URL and therefore the bot token: do not print it.
		t.warn("request failed (network error or timeout)")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return
	}
	var apiErr struct {
		Description string `json:"description"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = json.Unmarshal(body, &apiErr)
	if apiErr.Description == "" {
		apiErr.Description = resp.Status
	}
	t.warn(apiErr.Description)
}

// warn reports the first delivery problem only, so a bad token or chat id is
// visible without flooding the log.
func (t *TelegramNotifier) warn(reason string) {
	t.warnOnce.Do(func() {
		log.Printf("telegram: notification not delivered: %s (further failures are not logged)", reason)
	})
}

// formatTelegram renders a console event as the polarity glyph, a bold title,
// and a sentence (parse_mode HTML). The glyph is the same one the console and
// the chart use, and it is sent even when console emojis are turned off.
func formatTelegram(event constants.Event, message string) string {
	title := eventGlyphPrefix(event) + "<b>" + htmlEscape(eventTitle(event)) + "</b>"
	body := Sentence(message)
	if body == "" || strings.EqualFold(body, eventTitle(event)) {
		return limitRunes(title, telegramTextLimit)
	}
	escaped := htmlEscape(body)
	budget := telegramTextLimit - len([]rune(title)) - 1
	if budget < 1 {
		return limitRunes(title, telegramTextLimit)
	}
	return title + "\n" + limitEscaped(escaped, budget)
}
