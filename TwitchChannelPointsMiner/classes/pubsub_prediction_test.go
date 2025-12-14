package classes

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

type stubPubSubLogger struct {
	printfCalls int
	errorCalls  int
	emojiCalls  int
	debugCalls  int
}

func (s *stubPubSubLogger) Printf(string, ...interface{}) {
	s.printfCalls++
}

func (s *stubPubSubLogger) Errorf(string, ...interface{}) {
	s.errorCalls++
}

func (s *stubPubSubLogger) EmojiPrintf(string, string, ...interface{}) {
	s.emojiCalls++
}

func (s *stubPubSubLogger) Debugf(string, ...interface{}) {
	s.debugCalls++
}

func (s *stubPubSubLogger) DebugEnabled() bool {
	return false
}

func TestPredictionEventDecideDoesNotMarkBetPlaced(t *testing.T) {
	streamer := &entities.Streamer{
		Username:      "tester",
		ChannelPoints: 1_000,
		Settings: entities.StreamerSettings{
			Bet: entities.BetSettings{Strategy: entities.StrategyMostVoted},
		},
	}
	event := NewPredictionEvent(streamer, map[string]interface{}{
		"id":     "ev1",
		"status": "ACTIVE",
		"title":  "test",
		"outcomes": []interface{}{
			map[string]interface{}{"id": "a", "title": "A", "color": "blue", "total_users": 10, "total_points": 100},
			map[string]interface{}{"id": "b", "title": "B", "color": "pink", "total_users": 5, "total_points": 50},
		},
	})
	if event == nil {
		t.Fatalf("expected event")
	}

	_ = event.Decide(streamer.ChannelPoints)
	if event.BetPlaced {
		t.Fatalf("Decide should not mark BetPlaced=true; BetPlaced is reserved for successful MakePrediction")
	}
}

func TestPlacePredictionStopsTrackingOnFilterSkip(t *testing.T) {
	logger := &stubPubSubLogger{}
	value := 1_000_000.0
	streamer := &entities.Streamer{
		Username:      "tester",
		ChannelPoints: 1_000,
		Settings: entities.StreamerSettings{
			Bet: entities.BetSettings{
				Strategy: entities.StrategyMostVoted,
				FilterCondition: &entities.FilterCondition{
					By:    entities.OutcomeTotalUsers,
					Where: entities.ConditionGT,
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

