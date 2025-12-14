package classes

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/privacy"

	"github.com/gorilla/websocket"
)

type Logger interface {
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	EmojiPrintf(emoji, format string, args ...interface{})
	Debugf(format string, args ...interface{})
	DebugEnabled() bool
}

var ErrPubSubReconnectRequested = errors.New("pubsub reconnect requested")

type PubSubClient struct {
	twitch      *Twitch
	logger      Logger
	anonymizer  *privacy.Anonymizer
	streamers   []*entities.Streamer
	streamerMap map[string]*entities.Streamer
	predictions map[string]*PredictionEvent
	predMu      sync.Mutex
	onGain      func(streamer *entities.Streamer, earned int, reason string, balance int)
	onPresence  func(streamer *entities.Streamer, online bool, reason string)
}

func (p *PubSubClient) anonymizeLogs() bool {
	return p != nil && p.anonymizer != nil && p.anonymizer.Enabled()
}

func (p *PubSubClient) streamerName(streamer *entities.Streamer) string {
	if streamer == nil {
		return ""
	}
	if p.anonymizeLogs() {
		return p.anonymizer.StreamerName(streamer)
	}
	return streamer.Username
}

func (p *PubSubClient) name(raw string) string {
	if p.anonymizeLogs() {
		return p.anonymizer.Name(raw)
	}
	return raw
}

func (p *PubSubClient) debugf(format string, args ...interface{}) {
	if p.logger != nil && p.logger.DebugEnabled() {
		p.logger.Debugf(format, args...)
	}
}

func (p *PubSubClient) deepDebugf(format string, args ...interface{}) {
	if p == nil || p.anonymizeLogs() {
		return
	}
	if p.logger == nil {
		return
	}
	deep, ok := p.logger.(deepDebugLogger)
	if !ok || !deep.DeepDebugEnabled() {
		return
	}
	deep.DeepDebugf(format, args...)
}

func NewPubSubClient(
	twitch *Twitch,
	logger Logger,
	anonymizer *privacy.Anonymizer,
	streamers []*entities.Streamer,
	onGain func(*entities.Streamer, int, string, int),
	onPresence func(*entities.Streamer, bool, string),
) *PubSubClient {
	streamerMap := make(map[string]*entities.Streamer)
	for _, s := range streamers {
		if s.ChannelID != "" {
			streamerMap[s.ChannelID] = s
		}
	}
	return &PubSubClient{
		twitch:      twitch,
		logger:      logger,
		anonymizer:  anonymizer,
		streamers:   streamers,
		streamerMap: streamerMap,
		predictions: make(map[string]*PredictionEvent),
		onGain:      onGain,
		onPresence:  onPresence,
	}
}

func (p *PubSubClient) Start(stop <-chan struct{}) {
	topics, err := p.buildTopics()
	if err != nil {
		p.logger.Errorf("PubSub topic error: %v", err)
		return
	}
	go p.pollPendingClaims(stop)
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
			if errors.Is(err, ErrPubSubReconnectRequested) {
				p.logger.Printf("PubSub[%d] reconnect requested; waiting ~60 seconds", connIndex)
				time.Sleep(60 * time.Second)
				continue
			}
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
				case readErr <- fmt.Errorf("PubSub[%d] server requested reconnect: %w", connIndex, ErrPubSubReconnectRequested):
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
						if p != nil && p.twitch != nil && p.twitch.twitchLogin != nil {
							username = strings.TrimSpace(p.twitch.twitchLogin.Username)
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
	topic, _ := data["topic"].(string)
	messageStr, _ := data["message"].(string)
	if messageStr == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(messageStr), &payload); err != nil {
		return err
	}
	msgType := strings.ToLower(fmt.Sprint(payload["type"]))
	prefix := topic
	if parts := strings.SplitN(topic, ".", 2); len(parts) > 0 {
		prefix = parts[0]
	}
	p.debugf("PubSub topic %s type=%s", prefix, msgType)
	p.deepDebugf("PubSub topic %s payload %s", topic, strings.TrimSpace(messageStr))
	channelID := channelIDFromPayload(payload, topic)

	switch {
	case msgType == "points-earned":
		return p.processPointsEarned(payload, channelID)
	case msgType == "claim-available":
		return p.processClaimAvailable(payload, channelID)
	case strings.HasPrefix(topic, "video-playback-by-id."):
		return p.processPlaybackMessage(topic, payload)
	case strings.HasPrefix(topic, "raid."):
		return p.processRaidMessage(topic, payload)
	case strings.HasPrefix(topic, "community-moments-channel-v1."):
		return p.processMomentMessage(topic, payload)
	case strings.HasPrefix(topic, "predictions-channel-v1."):
		return p.processPredictionChannel(topic, payload)
	case strings.HasPrefix(topic, "predictions-user-v1."):
		return p.processPredictionUser(payload)
	case strings.HasPrefix(topic, "community-points-channel-v1."):
		return p.processCommunityPointChannel(topic, payload)
	default:
		return nil
	}
}

func (p *PubSubClient) processPlaybackMessage(topic string, payload map[string]interface{}) error {
	channelID := ""
	if strings.HasPrefix(topic, "video-playback-by-id.") {
		channelID = strings.TrimPrefix(topic, "video-playback-by-id.")
	}
	if channelID == "" {
		return nil
	}
	streamer, ok := p.streamerMap[channelID]
	if !ok {
		return nil
	}
	if p.onPresence == nil {
		return nil
	}
	msgType := strings.ToLower(fmt.Sprint(payload["type"]))
	switch msgType {
	case "stream-up":
		if streamer.Stream == nil {
			streamer.Stream = entities.NewStream()
		} else {
			streamer.Stream.WatchStreakMissing = true
			streamer.Stream.ResetWatchProgress()
		}
		streamer.Stream.StreamUpAt = time.Now()
		p.onPresence(streamer, true, msgType)
	case "viewcount":
		if streamer.Stream != nil && !streamer.Stream.StreamUpElapsed() {
			return nil
		}
		if !streamer.OfflineAt.IsZero() && time.Since(streamer.OfflineAt) < time.Minute {
			return nil
		}
		live, err := p.twitch.IsStreamLive(streamer.ChannelID)
		if err != nil {
			p.logger.Errorf("live check %s: %v", p.streamerName(streamer), err)
			return nil
		}
		if live {
			p.onPresence(streamer, true, msgType)
		}
	case "stream-down":
		p.onPresence(streamer, false, msgType)
	}
	return nil
}

func (p *PubSubClient) processRaidMessage(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "raid.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.FollowRaid {
		return nil
	}
	raidData, _ := payload["raid"].(map[string]interface{})
	raidID, _ := raidData["id"].(string)
	target, _ := raidData["target_login"].(string)
	if raidID == "" {
		return nil
	}
	if streamer.LastRaidID == raidID {
		return nil
	}
	streamer.LastRaidID = raidID
	if err := p.twitch.JoinRaid(streamer, raidID); err != nil {
		p.logger.Errorf("join raid %s->%s: %v", p.streamerName(streamer), p.name(target), err)
		return nil
	}
	if target == "" {
		target = "raid target"
	}
	p.logger.EmojiPrintf(":performing_arts:", "Joining raid from %s to %s", p.streamerName(streamer), p.name(target))
	return nil
}

func (p *PubSubClient) processMomentMessage(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "community-moments-channel-v1.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.ClaimMoments {
		return nil
	}
	if strings.ToLower(fmt.Sprint(payload["type"])) != "active" {
		return nil
	}
	data, _ := payload["data"].(map[string]interface{})
	momentID, _ := data["moment_id"].(string)
	if momentID == "" {
		return nil
	}
	if err := p.twitch.ClaimMoment(streamer, momentID); err != nil {
		p.logger.Errorf("claim moment %s: %v", p.streamerName(streamer), err)
		return nil
	}
	p.logger.EmojiPrintf(":video_camera:", "%s Claimed Moment", p.streamerName(streamer))
	return nil
}

func (p *PubSubClient) processPointsEarned(payload map[string]interface{}, channelID string) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	if channelID == "" {
		channelID = channelIDFromPayload(payload, "")
	}
	if channelID == "" {
		return nil
	}
	streamer := p.streamerMap[channelID]
	if streamer == nil {
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

func (p *PubSubClient) processClaimAvailable(payload map[string]interface{}, channelID string) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	claim, _ := data["claim"].(map[string]interface{})
	claimID := stringOrDefault(claim["id"])

	if channelID == "" && claim != nil {
		channelID = stringValue(claim["channel_id"])
	}
	if channelID == "" {
		channelID = stringValue(data["channel_id"])
	}
	if channelID == "" {
		if balanceID := navigate(data, "balance.channel_id"); balanceID != nil {
			channelID = stringValue(balanceID)
		}
	}
	if channelID == "" && len(p.streamerMap) == 1 {
		for id := range p.streamerMap {
			channelID = id
		}
	}
	streamer := p.streamerMap[channelID]
	if streamer == nil || claimID == "" {
		if p.anonymizeLogs() {
			p.logger.Errorf("claim-available ignored: [redacted]")
		} else {
			p.logger.Errorf("claim-available ignored: channel=%s claim=%s streamer=%v", channelID, claimID, streamer != nil)
			p.deepDebugf("claim-available payload: %v", payload)
		}
		return nil
	}
	// p.logger.EmojiPrintf(":gift:", "Claim bonus for %s (claim %s, channel %s)", streamer.Username, claimID, channelID)
	if err := p.twitch.ClaimBonus(streamer, claimID); err != nil {
		if p.anonymizeLogs() {
			p.logger.Errorf("claim bonus %s: %v", p.streamerName(streamer), err)
		} else {
			p.logger.Errorf("claim bonus %s (channel %s): %v", streamer.Username, channelID, err)
		}
		return nil
	}
	// p.logger.Printf("Claim bonus success for %s (claim %s)", streamer.Username, claimID)
	return nil
}

func (p *PubSubClient) processPredictionChannel(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "predictions-channel-v1.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.MakePredictions {
		return nil
	}
	data, _ := payload["data"].(map[string]interface{})
	eventMap, _ := data["event"].(map[string]interface{})
	if eventMap == nil {
		return nil
	}
	eventID, _ := eventMap["id"].(string)
	status := strings.ToUpper(stringOrDefault(eventMap["status"]))

	msgType := strings.ToLower(fmt.Sprint(payload["type"]))

	switch msgType {
	case "event-created":
		if status != "ACTIVE" || eventID == "" {
			return nil
		}
		window := eventMap["prediction_window_seconds"]
		eventMap["prediction_window_seconds"] = streamer.PredictionWindowSeconds(fromFloat(window))
		event := NewPredictionEvent(streamer, eventMap)
		if event == nil {
			return nil
		}
		if streamer.Settings.Bet.MinimumPoints != nil && streamer.ChannelPoints <= *streamer.Settings.Bet.MinimumPoints {
			return nil
		}
		wait := event.ClosingAfter(time.Now())
		p.predMu.Lock()
		p.predictions[event.EventID] = event
		p.predMu.Unlock()
		time.AfterFunc(wait, func() {
			p.placePrediction(event.EventID)
		})
		p.logger.EmojiPrintf(":alarm_clock:", "Place bet after %s for %s", wait.Truncate(time.Second), p.streamerName(streamer))
	case "event-updated":
		var existing *PredictionEvent
		p.predMu.Lock()
		if ev, ok := p.predictions[eventID]; ok {
			existing = ev
			existing.Status = status
			if outcomes, ok := eventMap["outcomes"].([]interface{}); ok {
				existing.UpdateOutcomes(outcomes)
			}
		}
		p.predMu.Unlock()
		if existing != nil {
			p.resolvePredictionFromChannel(existing, eventMap)
		}
	}
	return nil
}

func (p *PubSubClient) processPredictionUser(payload map[string]interface{}) error {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return nil
	}
	predictionData, _ := data["prediction"].(map[string]interface{})
	if predictionData == nil {
		return nil
	}
	eventID := fmt.Sprint(predictionData["event_id"])
	p.predMu.Lock()
	event, ok := p.predictions[eventID]
	p.predMu.Unlock()
	if !ok || event == nil {
		return nil
	}

	switch strings.ToLower(fmt.Sprint(payload["type"])) {
	case "prediction-made":
		event.BetConfirmed = true
	case "prediction-result":
		result, _ := predictionData["result"].(map[string]interface{})
		if result == nil {
			return nil
		}
		if !event.BetConfirmed {
			// ? Assume confirmation if Twitch skipped sending prediction-made
			event.BetConfirmed = true
		}
		p.logPredictionResult(event, result)
		p.predMu.Lock()
		delete(p.predictions, eventID)
		p.predMu.Unlock()
	}
	return nil
}

func (p *PubSubClient) placePrediction(eventID string) {
	p.predMu.Lock()
	event, ok := p.predictions[eventID]
	p.predMu.Unlock()
	if !ok || event == nil || event.Streamer == nil {
		return
	}
	streamer := event.Streamer
	if event.Status != "ACTIVE" {
		p.logger.Printf("Skip bet for %s: event status is %s", p.streamerName(streamer), event.Status)
		return
	}
	if streamer.Settings.Bet.MinimumPoints != nil && streamer.ChannelPoints <= *streamer.Settings.Bet.MinimumPoints {
		if p.anonymizeLogs() {
			p.logger.Printf("Skip bet for %s: balance below minimum_points", p.streamerName(streamer))
		} else {
			p.logger.Printf("Skip bet for %s: balance %d <= minimum_points %d", streamer.Username, streamer.ChannelPoints, *streamer.Settings.Bet.MinimumPoints)
		}
		return
	}
	decision := event.Decide(streamer.ChannelPoints)
	if decision.OutcomeID == "" {
		p.logger.Printf("Skip bet for %s: no outcome selected", p.streamerName(streamer))
		return
	}
	if skip, compared, reason := event.ShouldSkipByFilter(); skip {
		if reason == "" {
			if p.anonymizeLogs() {
				reason = "filter_condition not satisfied"
			} else {
				reason = fmt.Sprintf("filter_condition not satisfied (current %s)", formatFloat(compared))
			}
		}
		if p.anonymizeLogs() {
			p.logger.Printf("Skip bet for %s: %s", p.streamerName(streamer), reason)
			return
		}
		p.logger.Printf("Skip bet for %s: %s", streamer.Username, reason)
		return
	}
	if decision.Amount < 10 {
		reason := fmt.Sprintf("balance %d below Twitch minimum 10", streamer.ChannelPoints)
		if streamer.ChannelPoints >= 10 {
			if streamer.Settings.Bet.MaxPoints != nil && *streamer.Settings.Bet.MaxPoints < 10 {
				reason = fmt.Sprintf("max_points %d below Twitch minimum 10", *streamer.Settings.Bet.MaxPoints)
			} else {
				reason = fmt.Sprintf("calculated stake %d below Twitch minimum", decision.Amount)
			}
		}
		if p.anonymizeLogs() {
			p.logger.Printf("Skip bet for %s: below Twitch minimum", p.streamerName(streamer))
		} else {
			p.logger.Printf("Skip bet for %s: %s", streamer.Username, reason)
		}
		return
	}
	if err := p.twitch.MakePrediction(event); err != nil {
		p.logger.Errorf("prediction %s: %v", p.streamerName(streamer), err)
		return
	}
	event.BetPlaced = true
	// Ensure we log results even if Twitch doesn't emit prediction-made
	event.BetConfirmed = true
	outcome := event.DecisionOutcomeString()
	if outcome == "" {
		outcome = decision.OutcomeID
	}
	if p.anonymizeLogs() {
		p.logger.EmojiPrintf(":four_leaf_clover:", "Place bet on: %s for %s", outcome, p.streamerName(streamer))
	} else {
		p.logger.EmojiPrintf(":four_leaf_clover:", "Place %s points on: %s for %s", formatNumber(decision.Amount), outcome, streamer.Username)
	}
	recordHistory(streamer, "PREDICTION", -decision.Amount)
}

func (p *PubSubClient) processCommunityPointChannel(topic string, payload map[string]interface{}) error {
	channelID := strings.TrimPrefix(topic, "community-points-channel-v1.")
	streamer := p.streamerMap[channelID]
	if streamer == nil || !streamer.Settings.CommunityGoals {
		return nil
	}
	msgType := strings.ToLower(fmt.Sprint(payload["type"]))
	if streamer.CommunityGoals == nil {
		streamer.CommunityGoals = make(map[string]*entities.CommunityGoal)
	}
	switch msgType {
	case "community-goal-created", "community-goal-updated":
		data, _ := payload["data"].(map[string]interface{})
		goalData, _ := data["community_goal"].(map[string]interface{})
		if goal := entities.NewCommunityGoalFromPubSub(goalData); goal != nil && goal.ID != "" {
			streamer.CommunityGoals[goal.ID] = goal
		}
		p.twitch.ContributeToCommunityGoals(streamer)
	case "community-goal-deleted":
		data, _ := payload["data"].(map[string]interface{})
		goalData, _ := data["community_goal"].(map[string]interface{})
		id := stringOrDefault(goalData["id"])
		if id != "" {
			delete(streamer.CommunityGoals, id)
		}
	}
	return nil
}

func (p *PubSubClient) randomPingInterval() time.Duration {
	return time.Duration(randomInt(25, 30)) * time.Second
}

// ? pollPendingClaims proactively checks each streamer for any outstanding community-point bonuses.
func (p *PubSubClient) pollPendingClaims(stop <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	p.checkPendingClaims()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.checkPendingClaims()
		}
	}
}

func (p *PubSubClient) checkPendingClaims() {
	for _, streamer := range p.streamers {
		if streamer == nil || streamer.Username == "" || streamer.ChannelID == "" {
			continue
		}
		if p.anonymizeLogs() {
			p.debugf("Checking pending bonus for %s", p.streamerName(streamer))
		} else {
			p.debugf("Checking pending bonus for %s (%s)", streamer.Username, streamer.ChannelID)
		}
		if _, err := p.twitch.LoadChannelPointsContext(streamer); err != nil {
			p.logger.Errorf("pending bonus check %s: %v", p.streamerName(streamer), err)
		}
	}
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

func stringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	val := strings.TrimSpace(fmt.Sprint(v))
	if val == "" || val == "<nil>" {
		return ""
	}
	return val
}

// ? claim/watch events are routed even when Twitch omits certain fields.
func channelIDFromPayload(payload map[string]interface{}, topic string) string {
	data, _ := payload["data"].(map[string]interface{})
	if data != nil {
		if prediction, ok := data["prediction"].(map[string]interface{}); ok {
			if id := stringValue(prediction["channel_id"]); id != "" {
				return id
			}
		}
		if claim, ok := data["claim"].(map[string]interface{}); ok {
			if id := stringValue(claim["channel_id"]); id != "" {
				return id
			}
		}
		if id := stringValue(data["channel_id"]); id != "" {
			return id
		}
		if balance, ok := data["balance"].(map[string]interface{}); ok {
			if id := stringValue(balance["channel_id"]); id != "" {
				return id
			}
		}
	}
	if topic != "" {
		if parts := strings.Split(topic, "."); len(parts) == 2 {
			if id := strings.TrimSpace(parts[1]); id != "" {
				return id
			}
		}
	}
	return ""
}

func recordHistory(streamer *entities.Streamer, reason string, amount int) {
	if streamer == nil || reason == "" {
		return
	}
	if streamer.History == nil {
		streamer.History = make(map[string]*entities.HistoryEntry)
	}
	entry, ok := streamer.History[reason]
	if !ok {
		entry = &entities.HistoryEntry{}
		streamer.History[reason] = entry
	}
	entry.Count++
	entry.Amount += amount
}

func (p *PubSubClient) logPredictionResult(event *PredictionEvent, result map[string]interface{}) {
	if event == nil || result == nil {
		return
	}
	gained, placed, won, resultType, resultString := event.ParseResult(result)
	event.BetConfirmed = true
	decisionLabel := event.DecisionLabel()
	if decisionLabel == "" {
		decisionLabel = fmt.Sprintf("Decision #%d", event.Decision.Choice+1)
	}
	color := constants.ColorGreen
	if gained < 0 {
		color = constants.ColorRed
	} else if gained == 0 {
		color = constants.ColorReset
	}
	label := event.String()
	if p.anonymizeLogs() {
		name := ""
		if event.Streamer != nil {
			name = p.streamerName(event.Streamer)
		}
		if name != "" && event.Title != "" {
			label = fmt.Sprintf("EventPrediction: %s - %s", name, event.Title)
		} else if name != "" {
			label = fmt.Sprintf("EventPrediction: %s", name)
		} else if event.Title != "" {
			label = fmt.Sprintf("EventPrediction: %s", event.Title)
		} else {
			label = "EventPrediction"
		}
		resultString = resultType
	}
	p.logger.EmojiPrintf(
		":bar_chart:",
		"%s - Decision: %s - Result: %s%s%s",
		label,
		decisionLabel,
		color,
		resultString,
		constants.ColorReset,
	)
	if streamer := event.Streamer; streamer != nil {
		if gained != 0 {
			recordHistory(streamer, "PREDICTION", gained)
		}
		if resultType == "REFUND" && placed > 0 {
			recordHistory(streamer, "REFUND", -placed)
		} else if resultType == "WIN" && won > 0 {
			recordHistory(streamer, "PREDICTION", -won)
		}
	}
}

func (p *PubSubClient) resolvePredictionFromChannel(event *PredictionEvent, eventMap map[string]interface{}) {
	if event == nil || event.Decision.Amount == 0 || event.ResultType != "" {
		return
	}
	status := strings.ToUpper(stringOrDefault(eventMap["status"]))
	if status != "RESOLVED" && status != "CANCELED" && status != "CANCELLED" {
		return
	}

	winningID := winningOutcomeID(eventMap)
	resultType := "LOSE"
	pointsWon := 0

	switch status {
	case "CANCELED", "CANCELLED":
		resultType = "REFUND"
	default:
		if winningID == "" {
			return
		}
		if event.Decision.OutcomeID == winningID {
			resultType = "WIN"
			pointsWon = payoutForOutcome(event.Decision, event.Outcomes, winningID)
		}
	}

	p.logPredictionResult(event, map[string]interface{}{
		"type":       resultType,
		"points_won": pointsWon,
	})

	p.predMu.Lock()
	delete(p.predictions, event.EventID)
	p.predMu.Unlock()
}

func winningOutcomeID(event map[string]interface{}) string {
	if id := stringOrDefault(event["winning_outcome_id"]); id != "" {
		return id
	}
	if outcomes, ok := event["outcomes"].([]interface{}); ok {
		for _, raw := range outcomes {
			oc, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if win, ok := oc["is_winning_outcome"].(bool); ok && win {
				if id := stringOrDefault(oc["id"]); id != "" {
					return id
				}
			}
			state := strings.ToUpper(stringOrDefault(oc["state"]))
			if state == "RESOLVED" || state == "WINNER" || state == "WIN" {
				if id := stringOrDefault(oc["id"]); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

func payoutForOutcome(decision PredictionDecision, outcomes []PredictionOutcome, winningID string) int {
	if decision.Amount <= 0 || decision.OutcomeID != winningID {
		return 0
	}

	totalPoints := 0
	winPoints := 0
	for _, oc := range outcomes {
		totalPoints += oc.TotalPoints
		if oc.ID == winningID {
			winPoints = oc.TotalPoints
		}
	}

	if totalPoints == 0 || winPoints == 0 {
		return decision.Amount
	}

	payout := int(math.Round(float64(decision.Amount) * (float64(totalPoints) / float64(winPoints))))
	if payout < decision.Amount {
		return decision.Amount
	}
	return payout
}
