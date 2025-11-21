package classes

import (
	"encoding/json"
	"fmt"
	"strings"
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
	streamers   []*entities.Streamer
	streamerMap map[string]*entities.Streamer
	onGain      func(streamer *entities.Streamer, earned int, reason string, balance int)
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
		streamers:   streamers,
		streamerMap: streamerMap,
		onGain:      onGain,
	}
}

func (p *PubSubClient) Start(stop <-chan struct{}) {
	topics, err := p.buildTopics()
	if err != nil {
		p.logger.Errorf("PubSub topic error: %v", err)
		return
	}
	batches := chunkTopics(topics, 50)
	for i, batch := range batches {
		idx := i + 1
		go p.run(idx, batch, stop)
	}
}

func (p *PubSubClient) run(connIndex int, topics []string, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}

		if err := p.connectAndListen(connIndex, topics, stop); err != nil {
			p.logger.Errorf("PubSub[%d] connection error: %v", connIndex, err)
			time.Sleep(10 * time.Second)
		}
	}
}

func (p *PubSubClient) connectAndListen(connIndex int, topics []string, stop <-chan struct{}) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(constants.WebsocketURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := p.listenTopics(conn, topics); err != nil {
		return err
	}

	p.logger.Printf("Connected to Twitch PubSub (conn #%d) with %d topic(s)", connIndex, len(topics))

	lastPong := time.Now()
	pingTimer := time.NewTimer(p.randomPingInterval())
	defer pingTimer.Stop()

	readErr := make(chan error, 1)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			if err := p.handleMessage(message, func() { lastPong = time.Now() }); err != nil {
				p.logger.Errorf("PubSub message error: %v", err)
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

func (p *PubSubClient) buildTopics() ([]string, error) {
	userID := p.twitch.twitchLogin.UserID()
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

func (p *PubSubClient) listenTopics(conn *websocket.Conn, topics []string) error {
	needsAuth := func(topic string) bool {
		return strings.HasPrefix(topic, "community-points-user-v1.") || strings.HasPrefix(topic, "predictions-user-v1.")
	}
	for _, t := range topics {
		data := map[string]interface{}{"topics": []string{t}}
		if needsAuth(t) {
			data["auth_token"] = p.twitch.twitchLogin.AuthToken()
		}
		payload := map[string]interface{}{
			"type":  "LISTEN",
			"nonce": randomString(16),
			"data":  data,
		}
		if err := conn.WriteJSON(payload); err != nil {
			return err
		}
	}
	return nil
}

func (p *PubSubClient) handleMessage(raw []byte, onPong func()) error {
	var envelope map[string]interface{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	typ, _ := envelope["type"].(string)
	switch typ {
	case "PONG":
		if onPong != nil {
			onPong()
		}
		return nil
	case "RESPONSE", "RECONNECT":
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

func (p *PubSubClient) randomPingInterval() time.Duration {
	return time.Duration(randomInt(25, 30)) * time.Second
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
