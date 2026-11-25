package pubsub

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/internal/random"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

func (p *Client) connectAndListen(connIndex int, topics []string, stop <-chan struct{}) error {
	dialer := *websocket.DefaultDialer
	if p.disableSSL {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	conn, _, err := dialer.Dial(twitchapi.WebsocketURL, nil)
	if err != nil {
		return err
	}

	lastPong := time.Now()
	pingTimer := time.NewTimer(p.randomPingInterval())
	defer pingTimer.Stop()

	msgCh := make(chan []byte, 256)
	workerCount := 4
	var workerWG sync.WaitGroup
	workerWG.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer workerWG.Done()
			for {
				select {
				case <-stop:
					return
				case raw, ok := <-msgCh:
					if !ok {
						return
					}
					if err := p.handleMessage(raw, nil); err != nil {
						p.logger.Errorf("PubSub message error: %v", err)
					}
				}
			}
		}()
	}

	var readWG sync.WaitGroup
	readWG.Add(1)

	defer func() {
		conn.Close()
		readWG.Wait()
		close(msgCh)
		workerWG.Wait()
	}()

	if err := p.listenTopics(conn, topics); err != nil {
		return err
	}

	p.logger.Printf("Connected to Twitch PubSub (conn #%d) with %d topic(s)", connIndex, len(topics))

	readErr := make(chan error, 1)
	go func() {
		defer readWG.Done()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case readErr <- err:
				default:
				}
				return
			}
			msgType := ""
			var envelope map[string]interface{}
			if err := json.Unmarshal(message, &envelope); err == nil {
				if t, ok := envelope["type"].(string); ok {
					msgType = strings.ToUpper(strings.TrimSpace(t))
				}
			}
			if msgType != "" {
				p.debugf("PubSub[%d] recv type=%s (%d bytes)", connIndex, msgType, len(message))
			} else {
				p.debugf("PubSub[%d] recv (%d bytes)", connIndex, len(message))
			}
			p.deepDebugf("PubSub[%d] recv: %s", connIndex, strings.TrimSpace(string(message)))
			if msgType == "PONG" {
				lastPong = time.Now()
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
				if respErr, ok := envelope["error"].(string); ok && strings.TrimSpace(respErr) != "" {
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
						cookieFile := filepath.Join("cookies", "<username>.json")
						if username != "" {
							cookieFile = filepath.Join("cookies", fmt.Sprintf("%s.json", username))
						}
						p.logger.Errorf("PubSub[%d] ERR_BADAUTH: most likely you have an outdated cookie file %q. Delete this file and try again.", connIndex, cookieFile)
					}
				}
			}

			select {
			case msgCh <- message:
			default:
				p.logger.Errorf("PubSub[%d] dropping message: worker queue full", connIndex)
			}
		}
	}()

	for {
		select {
		case <-stop:
			return nil
		case <-pingTimer.C:
			if err := conn.WriteJSON(map[string]string{"type": "PING"}); err != nil {
				return err
			}
			if time.Since(lastPong) > 5*time.Minute {
				return fmt.Errorf("last PONG >5m ago, reconnecting")
			}
			pingTimer.Reset(p.randomPingInterval())
		case err := <-readErr:
			return err
		}
	}
}

func (p *Client) buildTopics() ([]string, error) {
	userID := p.twitch.UserID()
	if userID == "" {
		return nil, fmt.Errorf("no user id for pubsub")
	}
	topics := []string{}
	seen := make(map[string]struct{})

	addTopic := func(topic string) {
		if topic == "" {
			return
		}
		if _, ok := seen[topic]; ok {
			return
		}
		seen[topic] = struct{}{}
		topics = append(topics, topic)
	}

	addTopic(fmt.Sprintf("community-points-user-v1.%s", userID))

	shouldListenPredictionUser := false
	for _, s := range p.streamers {
		if s.Settings.MakePredictions {
			shouldListenPredictionUser = true
			break
		}
	}
	if shouldListenPredictionUser {
		addTopic(fmt.Sprintf("predictions-user-v1.%s", userID))
	}

	for _, s := range p.streamers {
		if s.ChannelID == "" {
			continue
		}
		addTopic(fmt.Sprintf("video-playback-by-id.%s", s.ChannelID))
		if s.Settings.FollowRaid {
			addTopic(fmt.Sprintf("raid.%s", s.ChannelID))
		}
		if s.Settings.MakePredictions {
			addTopic(fmt.Sprintf("predictions-channel-v1.%s", s.ChannelID))
		}
		if s.Settings.ClaimMoments {
			addTopic(fmt.Sprintf("community-moments-channel-v1.%s", s.ChannelID))
		}
		if s.Settings.CommunityGoals {
			addTopic(fmt.Sprintf("community-points-channel-v1.%s", s.ChannelID))
		}
	}

	return topics, nil
}

func (p *Client) listenTopics(conn *websocket.Conn, topics []string) error {
	needsAuth := func(topic string) bool {
		return strings.HasPrefix(topic, "community-points-user-v1.") || strings.HasPrefix(topic, "predictions-user-v1.")
	}
	for _, t := range topics {
		data := map[string]interface{}{"topics": []string{t}}
		if needsAuth(t) {
			data["auth_token"] = p.twitch.AuthToken()
		}
		payload := map[string]interface{}{
			"type":  "LISTEN",
			"nonce": random.String(16),
			"data":  data,
		}
		if p.anonymizeLogs() {
			prefix := t
			if parts := strings.SplitN(t, ".", 2); len(parts) > 0 {
				prefix = parts[0]
			}
			p.debugf("PubSub LISTEN %s", prefix)
		} else {
			p.debugf("PubSub LISTEN %s", t)
		}
		if err := conn.WriteJSON(payload); err != nil {
			return err
		}
	}
	return nil
}

func (p *Client) randomPingInterval() time.Duration {
	return time.Duration(random.Int(25, 30)) * time.Second
}

func chunkTopics(topics []string, chunkSize int) [][]string {
	if chunkSize <= 0 {
		return [][]string{topics}
	}
	var batches [][]string
	for len(topics) > 0 {
		end := chunkSize
		if len(topics) < chunkSize {
			end = len(topics)
		}
		batches = append(batches, topics[:end])
		topics = topics[end:]
	}
	return batches
}
