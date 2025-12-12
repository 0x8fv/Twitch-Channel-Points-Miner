package classes

import "testing"

type stubChatLogger struct {
	calls []string
}

func (s *stubChatLogger) Printf(format string, args ...interface{}) {
	s.calls = append(s.calls, "printf")
}

func (s *stubChatLogger) Errorf(format string, args ...interface{}) {
	s.calls = append(s.calls, "errorf")
}

func (s *stubChatLogger) EmojiPrintf(emoji, format string, args ...interface{}) {
	s.calls = append(s.calls, "emoji:"+emoji)
}

func TestNewChatClientNormalizesInput(t *testing.T) {
	client := NewChatClient("UserName", " token ", "Channel ", nil, false)
	if client.username != "username" || client.channel != "channel" || client.token != "token" {
		t.Fatalf("normalization failed: %#v", client)
	}
}

func TestHandleLineMentionLogging(t *testing.T) {
	logger := &stubChatLogger{}
	client := NewChatClient("target", "token", "chan", logger, false)

	client.handleLine(":nick!user PRIVMSG #chan :hello @target there")
	if len(logger.calls) != 1 || logger.calls[0] != "emoji::speech_balloon:" {
		t.Fatalf("mention should log once, got %#v", logger.calls)
	}
}

func TestHandleLineDisableAtNickname(t *testing.T) {
	logger := &stubChatLogger{}
	client := NewChatClient("target", "token", "chan", logger, true)

	client.handleLine(":nick!u PRIVMSG #chan :target is cool")
	if len(logger.calls) != 1 {
		t.Fatalf("expected mention due to disableAtInNickname flag, got %#v", logger.calls)
	}
}
