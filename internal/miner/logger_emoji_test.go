package miner

import (
	"testing"

	"TwitchChannelPointsMiner/internal/events"
)

func TestEmojize(t *testing.T) {
	if got := emojize(":rocket:"); got == ":rocket:" {
		t.Fatalf("expected known emoji mapping, got %q", got)
	}
	if got := emojize(":unknown:"); got != ":unknown:" {
		t.Fatalf("unknown emoji should pass through, got %q", got)
	}
}

func TestEmojiForUsesEventPolarity(t *testing.T) {
	if got := emojiFor(":partying_face:", events.EventStreamerOnline); got != "🟢" {
		t.Fatalf("online glyph = %q", got)
	}
	if got := emojiFor(":sleeping:", events.EventStreamerOffline); got != "🔴" {
		t.Fatalf("offline glyph = %q", got)
	}
	if got := emojiFor(":bar_chart:", events.EventBetWin); got != "📈" {
		t.Fatalf("win glyph = %q", got)
	}
	if got := emojiFor(":bar_chart:", events.EventBetLose); got != "📉" {
		t.Fatalf("lose glyph = %q", got)
	}
	if got := emojiFor(":rocket:", events.EventGainForWatchStreak); got != "⏫" {
		t.Fatalf("streak glyph = %q", got)
	}
	if got := emojiFor(":rocket:", ""); got != "🚀" {
		t.Fatalf("fallback glyph = %q", got)
	}
	if got := emojiFor("", ""); got != "" {
		t.Fatalf("empty glyph = %q", got)
	}
}
