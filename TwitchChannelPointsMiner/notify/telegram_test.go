package notify

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
	if first.Get("chat_id") != "123456789" || first.Get("text") != "🟢 <b>Online</b>\nAlice is online." ||
		first.Get("parse_mode") != "HTML" || first.Get("disable_notification") != "true" ||
		first.Get("link_preview_options") != `{"is_disabled":true}` {
		t.Fatalf("unexpected form %v", first)
	}
	long := forms[1].Get("text")
	if !strings.HasPrefix(long, "📉 <b>Bet lost</b>\n") {
		t.Fatalf("long text missing title: %q", long[:40])
	}
	if got := len([]rune(long)); got > telegramTextLimit {
		t.Fatalf("long text should be at most %d characters, got %d", telegramTextLimit, got)
	}
}

func TestTelegramNotifierSkipsQuietEventsUnlessNamed(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = append(got, r.PostForm.Get("text"))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	open := newTelegramNotifier(TelegramSettings{ChatID: "1", Token: "t"}, srv.URL)
	open.Send("tick", constants.EventGainForWatch)
	open.Send("Alice (1.2k points) is Online!", constants.EventStreamerOnline)
	if len(got) != 1 || !strings.Contains(got[0], "Alice") {
		t.Fatalf("empty events list should skip watch ticks and send online, got %v", got)
	}

	got = nil
	all := newTelegramNotifier(TelegramSettings{ChatID: "1", Token: "t", Events: []string{"*"}}, srv.URL)
	all.Send("tick", constants.EventGainForWatch)
	if len(got) != 1 || !strings.HasPrefix(got[0], "➕ <b>Watch</b>") {
		t.Fatalf("* should opt into watch ticks, got %v", got)
	}
}

func TestTelegramNotifierDisabledWithoutCredentials(t *testing.T) {
	if NewTelegramNotifier(TelegramSettings{ChatID: "1"}) != nil || NewTelegramNotifier(TelegramSettings{Token: "t"}) != nil {
		t.Fatal("notifier must be nil unless chat_id and token are both set")
	}
	var n *TelegramNotifier
	n.Send("ignored", constants.EventStartup) // nil notifier is a no-op
}

func TestFormatTelegram(t *testing.T) {
	for _, tc := range []struct {
		event   constants.Event
		message string
		want    string
	}{
		{constants.EventStreamerOnline, "Alice is Online!", "🟢 <b>Online</b>\nAlice is online."},
		{constants.EventBetLose, "EventPrediction: bob - overtime? - Decision: No - Result: LOSE, Gained: -500", "📉 <b>Bet lost</b>\nbob lost 500 points on \"overtime?\" after choosing No."},
	} {
		if got := formatTelegram(tc.event, tc.message); got != tc.want {
			t.Fatalf("formatTelegram(%s) got %q, want %q", tc.event, got, tc.want)
		}
	}
}
