package pubsub

import (
	"TwitchChannelPointsMiner/internal/events"
)

type stubLogger struct {
	printfCalls int
	errorCalls  int
	emojiCalls  int
	debugCalls  int
}

func (s *stubLogger) Printf(string, ...interface{}) {
	s.printfCalls++
}

func (s *stubLogger) Errorf(string, ...interface{}) {
	s.errorCalls++
}

func (s *stubLogger) EmojiPrintf(string, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubLogger) Eventf(events.Event, string, ...interface{}) {
	s.printfCalls++
}

func (s *stubLogger) EmojiEventf(string, events.Event, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubLogger) ErrorEventf(events.Event, string, ...interface{}) {
	s.errorCalls++
}

func (s *stubLogger) Debugf(string, ...interface{}) {
	s.debugCalls++
}

func (s *stubLogger) DebugEnabled() bool {
	return false
}
