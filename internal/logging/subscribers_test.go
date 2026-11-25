package logging

import (
	"testing"

	"TwitchChannelPointsMiner/internal/ansi"
	"TwitchChannelPointsMiner/internal/events"
)

type published struct {
	event    events.Event
	sentence string
}

func TestLoggerPublishesEventsToSubscribers(t *testing.T) {
	var got []published
	Subscribe("test", func(event events.Event, sentence string) {
		got = append(got, published{event, sentence})
	})
	defer Subscribe("test", nil)
	last := func() published {
		t.Helper()
		if len(got) == 0 {
			t.Fatalf("nothing was published")
		}
		return got[len(got)-1]
	}

	logger := NewLogger(LoggerSettings{}, "")
	logger.Printf("plain lines are not published")
	if len(got) != 0 {
		t.Fatalf("non-event log line was published: %v", got)
	}
	logger.Eventf(events.EventStreamerOnline, "%sAlice%s is Online!", ansi.ColorGreen, ansi.ColorReset)
	if p := last(); p.sentence != "Alice is online." || p.event != events.EventStreamerOnline {
		t.Fatalf("event should arrive as a plain sentence without colour codes, got %+v", p)
	}

	// The console glyph is added after publishing, so it never leaks into a notification.
	logger = NewLogger(LoggerSettings{Emoji: true}, "")
	logger.EmojiEventf(":partying_face:", events.EventStreamerOnline, "%sAlice%s is Online!", ansi.ColorGreen, ansi.ColorReset)
	if p := last(); p.sentence != "Alice is online." {
		t.Fatalf("notification should not carry the console shortcode, got %+v", p)
	}
	logger.EmojiEventf(":bar_chart:", events.EventBetLose, "EventPrediction: bob - overtime? - Decision: No - Result: %sLOSE%s, Gained: -500", ansi.ColorRed, ansi.ColorReset)
	if p := last(); p.sentence != "bob lost 500 points on \"overtime?\" after choosing No." || p.event != events.EventBetLose {
		t.Fatalf("loss notification should stay a sentence, got %+v", p)
	}

	Subscribe("test", nil)
	before := len(got)
	logger.Eventf(events.EventStreamerOnline, "Alice is Online!")
	if len(got) != before {
		t.Fatalf("removed subscriber still received an event")
	}
}
