package twitchchannelpointsminer

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

type sentEvent struct {
	message string
	event   constants.Event
}

type recordingSender struct {
	sent []sentEvent
}

func (r *recordingSender) Send(message string, event constants.Event) {
	r.sent = append(r.sent, sentEvent{message, event})
}

func TestLoggerForwardsEventsToTelegram(t *testing.T) {
	sender := &recordingSender{}
	setTelegramSender(sender)
	defer telegramSender.Store(nil)
	last := func() sentEvent {
		t.Helper()
		if len(sender.sent) == 0 {
			t.Fatalf("nothing was sent")
		}
		return sender.sent[len(sender.sent)-1]
	}

	logger := NewLogger(LoggerSettings{}, "")
	logger.Printf("plain lines are not sent")
	if len(sender.sent) != 0 {
		t.Fatalf("non-event log line was sent: %v", sender.sent)
	}
	logger.Eventf(constants.EventStreamerOnline, "%sAlice%s is Online!", colorGreen, colorReset)
	if got := last(); got.message != "Alice is online." || got.event != constants.EventStreamerOnline {
		t.Fatalf("event should arrive as a plain sentence without colour codes, got %+v", got)
	}

	// The console glyph is added after the notification is sent, so it never leaks into it.
	logger = NewLogger(LoggerSettings{Emoji: true}, "")
	logger.EmojiEventf(":partying_face:", constants.EventStreamerOnline, "%sAlice%s is Online!", colorGreen, colorReset)
	if got := last(); got.message != "Alice is online." {
		t.Fatalf("notification should not carry the console shortcode, got %+v", got)
	}
	logger.EmojiEventf(":bar_chart:", constants.EventBetLose, "EventPrediction: bob - overtime? - Decision: No - Result: %sLOSE%s, Gained: -500", colorRed, colorReset)
	if got := last(); got.message != "bob lost 500 points on \"overtime?\" after choosing No." || got.event != constants.EventBetLose {
		t.Fatalf("loss notification should stay a sentence, got %+v", got)
	}
}
