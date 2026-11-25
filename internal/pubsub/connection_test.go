package pubsub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// pubSubServer is a local stand-in for Twitch PubSub. serve runs once per connection.
func pubSubServer(t *testing.T, serve func(conn *websocket.Conn)) *Client {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		serve(conn)
	}))
	t.Cleanup(srv.Close)
	return &Client{logger: &stubLogger{}, endpoint: "ws" + strings.TrimPrefix(srv.URL, "http")}
}

func waitFor(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("connection did not return")
		return nil
	}
}

func TestConnectAndListenSubscribesAndHonoursReconnect(t *testing.T) {
	listened := make(chan map[string]interface{}, 1)
	p := pubSubServer(t, func(conn *websocket.Conn) {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		listened <- msg
		_ = conn.WriteJSON(map[string]string{"type": "RECONNECT"})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	done := make(chan error, 1)
	go func() { done <- p.connectAndListen(1, []string{"video-playback-by-id.123"}, make(chan struct{})) }()

	if err := waitFor(t, done); !errors.Is(err, ErrReconnectRequested) {
		t.Fatalf("got %v, want a reconnect request", err)
	}
	msg := <-listened
	data, _ := msg["data"].(map[string]interface{})
	topics, _ := data["topics"].([]interface{})
	if msg["type"] != "LISTEN" || len(topics) != 1 || topics[0] != "video-playback-by-id.123" {
		t.Fatalf("unexpected subscription message %v", msg)
	}
	if _, hasToken := data["auth_token"]; hasToken {
		t.Fatalf("a public topic should not carry the auth token")
	}
}

func TestConnectAndListenStopsCleanly(t *testing.T) {
	p := pubSubServer(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- p.connectAndListen(1, nil, stop) }()
	time.Sleep(100 * time.Millisecond)
	close(stop)

	if err := waitFor(t, done); err != nil {
		t.Fatalf("stopping should not be an error, got %v", err)
	}
}

func TestConnectAndListenReportsLostConnection(t *testing.T) {
	p := pubSubServer(t, func(conn *websocket.Conn) {
		// Dropping the connection right away is what a network failure looks like.
	})

	done := make(chan error, 1)
	go func() { done <- p.connectAndListen(1, nil, make(chan struct{})) }()

	if err := waitFor(t, done); err == nil || errors.Is(err, ErrReconnectRequested) {
		t.Fatalf("got %v, want a read error", err)
	}
}
