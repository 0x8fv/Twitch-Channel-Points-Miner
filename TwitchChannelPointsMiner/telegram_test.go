package twitchchannelpointsminer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

func TestTelegramNotifierSendsSelectedEvents(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var forms []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		forms = append(forms, r.PostForm)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	n := newTelegramNotifier(TelegramSettings{
		ChatID:              " 123456789 ",
		Token:               "42:secret",
		Events:              []string{"streamer_online", "BET_LOSE"},
		DisableNotification: true,
	}, srv.URL)
	if n == nil {
		t.Fatal("notifier should be enabled")
	}

	n.Send("Alice is Online!", constants.EventStreamerOnline)
	n.Send("+10 points", constants.EventGainForWatch) // not selected
	n.Send("no event", "")
	n.Send(strings.Repeat("é", 5000), constants.EventBetLose)

	if len(forms) != 2 {
		t.Fatalf("got %d requests, want 2", len(forms))
	}
	if paths[0] != "POST /bot42:secret/sendMessage" {
		t.Fatalf("unexpected request %q", paths[0])
	}
	first := forms[0]
	if first.Get("chat_id") != "123456789" || first.Get("text") != "Alice is Online!" ||
		first.Get("disable_notification") != "true" || first.Get("link_preview_options") != `{"is_disabled":true}` {
		t.Fatalf("unexpected form %v", first)
	}
	if got := len([]rune(forms[1].Get("text"))); got != telegramTextLimit {
		t.Fatalf("long text should be cut to %d characters, got %d", telegramTextLimit, got)
	}
}

func TestTelegramNotifierDisabledWithoutCredentials(t *testing.T) {
	if NewTelegramNotifier(TelegramSettings{ChatID: "1"}) != nil || NewTelegramNotifier(TelegramSettings{Token: "t"}) != nil {
		t.Fatal("notifier must be nil unless chat_id and token are both set")
	}
	var n *TelegramNotifier
	n.Send("ignored", constants.EventStartup) // nil notifier is a no-op
}

func TestLoggerForwardsEventsToTelegram(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostForm
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	telegramNotifier.Store(newTelegramNotifier(TelegramSettings{ChatID: "1", Token: "t"}, srv.URL))
	defer telegramNotifier.Store(nil)

	logger := NewLogger(LoggerSettings{}, "")
	logger.Printf("plain lines are not sent")
	if got != nil {
		t.Fatalf("non-event log line was sent: %v", got)
	}
	logger.Eventf(constants.EventStreamerOnline, "%sAlice%s is Online!", colorGreen, colorReset)
	if got.Get("text") != "Alice is Online!" {
		t.Fatalf("event should arrive ANSI-stripped, got %q", got.Get("text"))
	}
}
