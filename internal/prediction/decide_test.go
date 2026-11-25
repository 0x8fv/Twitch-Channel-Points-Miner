package prediction

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func TestPredictionEventDecideDoesNotMarkBetPlaced(t *testing.T) {
	streamer := &domain.Streamer{
		Username:      "tester",
		ChannelPoints: 1_000,
		Settings: domain.StreamerSettings{
			Bet: domain.BetSettings{Strategy: domain.StrategyMostVoted},
		},
	}
	event := NewEvent(streamer, map[string]interface{}{
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
