package pubsub

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/prediction"
)

func (p *Client) processPredictionChannel(topic string, payload map[string]interface{}) error {
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
	status := strings.ToUpper(jsonvalue.String(eventMap["status"]))

	msgType := strings.ToLower(fmt.Sprint(payload["type"]))

	switch msgType {
	case "event-created":
		if status != "ACTIVE" || eventID == "" {
			return nil
		}
		window := eventMap["prediction_window_seconds"]
		eventMap["prediction_window_seconds"] = streamer.PredictionWindowSeconds(jsonvalue.Float(window))
		event := prediction.NewEvent(streamer, eventMap)
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
		p.logger.EmojiEventf(":alarm_clock:", events.EventBetStart, "Place bet after %s for %s", wait.Truncate(time.Second), p.streamerName(streamer))
	case "event-updated":
		var existing *prediction.Event
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

func (p *Client) processPredictionUser(payload map[string]interface{}) error {
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
