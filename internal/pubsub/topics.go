package pubsub

import (
	"fmt"
	"github.com/gorilla/websocket"
	"strings"

	"TwitchChannelPointsMiner/internal/random"
)

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
