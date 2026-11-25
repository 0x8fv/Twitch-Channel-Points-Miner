package pubsub

import (
	"path/filepath"
	"testing"
)

func TestMessageType(t *testing.T) {
	for raw, want := range map[string]string{
		`{"type":"PONG"}`:              "PONG",
		`{"type":" reconnect "}`:       "RECONNECT",
		`{"type":"MESSAGE","data":{}}`: "MESSAGE",
		`{"data":{}}`:                  "",
		`{"type":5}`:                   "",
		`not json`:                     "",
	} {
		if got, _ := messageType([]byte(raw)); got != want {
			t.Fatalf("messageType(%s) got %q, want %q", raw, got, want)
		}
	}
	if _, envelope := messageType([]byte(`{"type":"RESPONSE","error":"ERR_BADAUTH"}`)); envelope["error"] != "ERR_BADAUTH" {
		t.Fatalf("decoded message should be returned, got %v", envelope)
	}
	if _, envelope := messageType([]byte(`not json`)); envelope != nil {
		t.Fatalf("undecodable message should have no envelope, got %v", envelope)
	}
}

func TestLogResponseError(t *testing.T) {
	logger := &stubLogger{}
	p := &Client{logger: logger}

	p.logResponseError(1, map[string]interface{}{"type": "RESPONSE", "error": "", "nonce": "abc"})
	p.logResponseError(1, map[string]interface{}{"type": "RESPONSE"})
	if logger.errorCalls != 0 {
		t.Fatalf("a response without an error should log nothing, got %d errors", logger.errorCalls)
	}
	p.logResponseError(1, map[string]interface{}{"type": "RESPONSE", "error": "ERR_BADTOPIC", "nonce": "abc"})
	if logger.errorCalls != 1 {
		t.Fatalf("an error response should log once, got %d", logger.errorCalls)
	}
	p.logResponseError(1, map[string]interface{}{"type": "RESPONSE", "error": "ERR_BADAUTH"})
	if logger.errorCalls != 3 {
		t.Fatalf("a bad auth response should log the error and the cookie hint, got %d errors in total", logger.errorCalls)
	}
}

func TestCookieFileOf(t *testing.T) {
	if got, want := cookieFileOf(""), filepath.Join("cookies", "<username>.json"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := cookieFileOf("alice"), filepath.Join("cookies", "alice.json"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
