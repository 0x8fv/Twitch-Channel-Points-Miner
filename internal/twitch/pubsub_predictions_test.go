package twitch

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestPlacePredictionStopsTrackingOnFilterSkip(t *testing.T) {
	logger := &stubPubSubLogger{}
	value := 1_000_000.0
	streamer := &domain.Streamer{
		Username:      "tester",
		ChannelPoints: 1_000,
		Settings: domain.StreamerSettings{
			Bet: domain.BetSettings{
				Strategy: domain.StrategyMostVoted,
				FilterCondition: &domain.FilterCondition{
					By:    domain.OutcomeTotalUsers,
					Where: domain.ConditionGT,
					Value: &value,
				},
			},
		},
	}
	event := NewPredictionEvent(streamer, map[string]interface{}{
		"id":     "ev-skip",
		"status": "ACTIVE",
		"title":  "skip-me",
		"outcomes": []interface{}{
			map[string]interface{}{"id": "a", "title": "A", "color": "blue", "total_users": 10, "total_points": 100},
			map[string]interface{}{"id": "b", "title": "B", "color": "pink", "total_users": 5, "total_points": 50},
		},
	})
	if event == nil {
		t.Fatalf("expected event")
	}

	client := &PubSubClient{
		logger:      logger,
		predictions: map[string]*PredictionEvent{event.EventID: event},
	}

	client.placePrediction(event.EventID)
	if _, ok := client.predictions[event.EventID]; ok {
		t.Fatalf("expected prediction %q to be removed from tracking after filter skip", event.EventID)
	}
	if event.ResultType != "SKIPPED" {
		t.Fatalf("expected ResultType SKIPPED, got %q", event.ResultType)
	}
}
