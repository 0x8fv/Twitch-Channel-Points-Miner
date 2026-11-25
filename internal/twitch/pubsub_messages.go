package twitch

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
