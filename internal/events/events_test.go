package events

import (
	"testing"
)

func TestEventGlyphsCoverEveryEvent(t *testing.T) {
	all := []Event{
		EventStartup, EventShutdown,
		EventStreamerOnline, EventStreamerOffline,
		EventGainForRaid, EventGainForClaim, EventGainForWatch, EventGainForWatchStreak,
		EventBetWin, EventBetLose, EventBetRefund, EventBetFilters, EventBetGeneral, EventBetFailed, EventBetStart,
		EventBonusClaim, EventMomentClaim, EventJoinRaid, EventDropClaim, EventDropStatus, EventChatMention,
	}
	if len(eventGlyphs) != len(all) {
		t.Fatalf("glyph map has %d entries, event list has %d", len(eventGlyphs), len(all))
	}
	pos, neg := 0, 0
	for _, event := range all {
		emoji, tone, ok := EventGlyph(event)
		if !ok || emoji == "" {
			t.Fatalf("missing glyph for %s", event)
		}
		switch tone {
		case TonePositive:
			pos++
		case ToneNegative:
			neg++
		case ToneNeutral:
		default:
			t.Fatalf("unknown tone %q for %s", tone, event)
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("expected both polarities, pos=%d neg=%d", pos, neg)
	}
	win, winTone, _ := EventGlyph(EventBetWin)
	lose, loseTone, _ := EventGlyph(EventBetLose)
	if win == lose || winTone != TonePositive || loseTone != ToneNegative {
		t.Fatalf("bet result glyphs: win=%s %s lose=%s %s", win, winTone, lose, loseTone)
	}
	on, onTone, _ := EventGlyph(EventStreamerOnline)
	off, offTone, _ := EventGlyph(EventStreamerOffline)
	if on == off || onTone != TonePositive || offTone != ToneNegative {
		t.Fatalf("presence glyphs: online=%s %s offline=%s %s", on, onTone, off, offTone)
	}
	if _, _, ok := EventGlyph(""); ok {
		t.Fatal("empty event should not have a glyph")
	}
}
