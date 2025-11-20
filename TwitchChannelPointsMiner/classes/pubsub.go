package classes

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"

	"github.com/gorilla/websocket"
)

type Logger interface {
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type PubSubClient struct {
	twitch      *Twitch
	logger      Logger
	streamerMap map[string]*entities.Streamer
	onGain      func(streamer *entities.Streamer, earned int, reason string, balance int)

	conn *websocket.Conn
	mu   sync.Mutex
}

func NewPubSubClient(twitch *Twitch, logger Logger, streamers []*entities.Streamer, onGain func(*entities.Streamer, int, string, int)) *PubSubClient {
	streamerMap := make(map[string]*entities.Streamer)
	for _, s := range streamers {
		if s.ChannelID != "" {
			streamerMap[s.ChannelID] = s
		}
	}
	return &PubSubClient{
		twitch:      twitch,
		logger:      logger,
		streamerMap: streamerMap,
		onGain:      onGain,
	}
}

func (p *PubSubClient) Start(stop <-chan struct{}) {
	go p.run(stop)
}

func (p *PubSubClient) run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			p.close()
			return
		default:
		}

		if err := p.connectAndListen(stop); err != nil {
			p.logger.Errorf("PubSub connection error: %v", err)
			time.Sleep(10 * time.Second)
		}
	}
}

func (p *PubSubClient) connectAndListen(stop <-chan struct{}) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(constants.WebsocketURL, nil)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.conn = conn
	p.mu.Unlock()
	defer p.close()

	if err := p.listenTopics(); err != nil {
		return err
	}

	p.logger.Printf("Connected to Twitch PubSub")

	pingTicker := time.NewTicker(4 * time.Minute)
	defer pingTicker.Stop()

	readErr := make(chan error, 1)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			if err := p.handleMessage(message); err != nil {
				p.logger.Errorf("PubSub message error: %v", err)
			}
		}
	}()

	for {
		select {
		case <-stop:
			return nil
		case <-pingTicker.C:
			p.conn.WriteJSON(map[string]string{"type": "PING"})
		case err := <-readErr:
			return err
		}
	}
}

func (p *PubSubClient) listenTopics() error {
	userID := p.twitch.twitchLogin.UserID()
	if userID == "" {
		return fmt.Errorf("no user id for pubsub")
	}
	topics := []string{fmt.Sprintf("community-points-user-v1.%s", userID)}
	payload := map[string]interface{}{
		"type":  "LISTEN",
		"nonce": randomString(16),
		"data": map[string]interface{}{
			"topics":     topics,
			"auth_token": p.twitch.twitchLogin.AuthToken(),
		},
	}
	return p.conn.WriteJSON(payload)
}

func (p *PubSubClient) handleMessage(raw []byte) error {
	var envelope map[string]interface{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	typ, _ := envelope["type"].(string)
	switch typ {
	case "RESPONSE", "PONG", "RECONNECT":
		return nil
	case "MESSAGE":
		return p.handleTopicMessage(envelope)
	default:
		return nil
	}
}

func (p *PubSubClient) handleTopicMessage(envelope map[string]interface{}) error {
	data, _ := envelope["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	messageStr, _ := data["message"].(string)
	if messageStr == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(messageStr), &payload); err != nil {
		return err
	}
	msgType, _ := payload["type"].(string)
	switch msgType {
	case "points-earned":
		return p.processPointsEarned(payload)
	default:
		return nil
	}
}

func (p *PubSubClient) processPointsEarned(payload map[string]interface{}) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	channelID := fmt.Sprint(data["channel_id"])
	if channelID == "" {
		return nil
	}
	streamer, ok := p.streamerMap[channelID]
	if !ok {
		return nil
	}
	pointGainVal := navigate(data, "point_gain")
	pointGain, _ := pointGainVal.(map[string]interface{})
	if pointGain == nil {
		return nil
	}
	reason := strings.ToUpper(fmt.Sprint(pointGain["reason_code"]))
	earned := int(fromFloat(pointGain["total_points"]))
	balance := streamer.ChannelPoints
	if balanceValue := navigate(data, "balance.balance"); balanceValue != nil {
		balance = int(fromFloat(balanceValue))
	}
	if p.onGain != nil {
		p.onGain(streamer, earned, reason, balance)
	}
	return nil
}

func (p *PubSubClient) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}
