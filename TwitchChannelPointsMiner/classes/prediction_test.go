package classes

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

func ptrFloat(val float64) *float64 { return &val }

func TestShouldSkipByFilterTotals(t *testing.T) {
	outcomes := []PredictionOutcome{
		{TotalUsers: 10, TotalPoints: 100},
		{TotalUsers: 15, TotalPoints: 200},
	}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			Bet: entities.BetSettings{
				FilterCondition: &entities.FilterCondition{
					By:    entities.OutcomeTotalUsers,
					Where: entities.ConditionGTE,
					Value: ptrFloat(20),
				},
			},
		},
	}
	event := &PredictionEvent{
		Streamer: streamer,
		Outcomes: outcomes,
		Decision: PredictionDecision{Choice: 1},
	}

	skip, compared, reason := event.ShouldSkipByFilter()
	if skip {
		t.Fatalf("expected bet allowed, got skip (compared %.0f, reason %s)", compared, reason)
	}
	if compared != 25 {
		t.Fatalf("expected compared total users 25, got %.0f", compared)
	}

	// ? force skip
	event.Streamer.Settings.Bet.FilterCondition.Value = ptrFloat(30)
	skip, compared, _ = event.ShouldSkipByFilter()
	if !skip {
		t.Fatalf("expected skip when total users below threshold")
	}
	if compared != 25 {
		t.Fatalf("expected compared total users 25, got %.0f", compared)
	}
}

func TestShouldSkipByFilterDecisionUsers(t *testing.T) {
	outcomes := []PredictionOutcome{
		{TotalUsers: 5, TotalPoints: 10},
		{TotalUsers: 50, TotalPoints: 20},
	}
	streamer := &entities.Streamer{
		Settings: entities.StreamerSettings{
			Bet: entities.BetSettings{
				FilterCondition: &entities.FilterCondition{
					By:    entities.OutcomeDecisionUsers,
					Where: entities.ConditionLT,
					Value: ptrFloat(40),
				},
			},
		},
	}
	event := &PredictionEvent{
		Streamer: streamer,
		Outcomes: outcomes,
		Decision: PredictionDecision{Choice: 1},
	}

	skip, compared, _ := event.ShouldSkipByFilter()
	if !skip {
		t.Fatalf("expected skip when decision users 50 !< 40")
	}
	if compared != 50 {
		t.Fatalf("expected decision users compared 50, got %.0f", compared)
	}

	// ? Loosen threshold to allow betting
	event.Streamer.Settings.Bet.FilterCondition.Where = entities.ConditionGTE
	event.Streamer.Settings.Bet.FilterCondition.Value = ptrFloat(50)
	skip, compared, reason := event.ShouldSkipByFilter()
	if skip {
		t.Fatalf("expected bet allowed, got skip (compared %.0f, reason %s)", compared, reason)
	}
}
