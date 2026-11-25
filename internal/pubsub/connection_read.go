package pubsub

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// readMessages reads from conn until it fails or the server asks for a reconnect,
// which it reports on readErr. PONGs refresh lastPong; everything else is queued
// on msgCh for the workers.
func (p *Client) readMessages(conn *websocket.Conn, connIndex int, msgCh chan<- []byte, readErr chan<- error, lastPong *atomic.Pointer[time.Time]) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			select {
			case readErr <- err:
			default:
			}
			return
		}
		msgType, envelope := messageType(message)
		if msgType != "" {
			p.debugf("PubSub[%d] recv type=%s (%d bytes)", connIndex, msgType, len(message))
		} else {
			p.debugf("PubSub[%d] recv (%d bytes)", connIndex, len(message))
		}
		p.deepDebugf("PubSub[%d] recv: %s", connIndex, strings.TrimSpace(string(message)))
		if msgType == "PONG" {
			now := time.Now()
			lastPong.Store(&now)
			continue
		}
		if msgType == "RECONNECT" {
			select {
			case readErr <- fmt.Errorf("PubSub[%d] server requested reconnect: %w", connIndex, ErrReconnectRequested):
			default:
			}
			return
		}
		if msgType == "RESPONSE" && envelope != nil {
			p.logResponseError(connIndex, envelope)
		}

		select {
		case msgCh <- message:
		default:
			p.logger.Errorf("PubSub[%d] dropping message: worker queue full", connIndex)
		}
	}
}

// messageType returns the upper-case "type" of a PubSub message with the decoded
// message, or "" when it has none.
func messageType(message []byte) (string, map[string]interface{}) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(message, &envelope); err != nil {
		return "", envelope
	}
	if t, ok := envelope["type"].(string); ok {
		return strings.ToUpper(strings.TrimSpace(t)), envelope
	}
	return "", envelope
}

// logResponseError reports the error of a RESPONSE message, if it carries one.
func (p *Client) logResponseError(connIndex int, envelope map[string]interface{}) {
	respErr, ok := envelope["error"].(string)
	if !ok || strings.TrimSpace(respErr) == "" {
		return
	}
	nonce := strings.TrimSpace(fmt.Sprint(envelope["nonce"]))
	if nonce != "" {
		p.logger.Errorf("PubSub[%d] RESPONSE error nonce=%s: %s", connIndex, nonce, respErr)
	} else {
		p.logger.Errorf("PubSub[%d] RESPONSE error: %s", connIndex, respErr)
	}
	if strings.Contains(respErr, "ERR_BADAUTH") {
		username := ""
		if p != nil && p.twitch != nil {
			username = strings.TrimSpace(p.twitch.LoginName())
		}
		p.logger.Errorf("PubSub[%d] ERR_BADAUTH: most likely you have an outdated cookie file %q. Delete this file and try again.", connIndex, cookieFileOf(username))
	}
}

// cookieFileOf is where the login of username keeps its cookies.
func cookieFileOf(username string) string {
	if username == "" {
		return filepath.Join("cookies", "<username>.json")
	}
	return filepath.Join("cookies", fmt.Sprintf("%s.json", username))
}
