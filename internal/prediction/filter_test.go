package prediction

import (
	"testing"

	"TwitchChannelPointsMiner/internal/domain"
)

func ptrFloat(val float64) *float64 { return &val }

func TestShouldSkipByFilterTotals(t *testing.T) {
	outcomes := []Outcome{
		{TotalUsers: 10, TotalPoints: 100},
		{TotalUsers: 15, TotalPoints: 200},
	}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			Bet: domain.BetSettings{
				FilterCondition: &domain.FilterCondition{
					By:    domain.OutcomeTotalUsers,
					Where: domain.ConditionGTE,
					Value: ptrFloat(20),
				},
			},
		},
	}
	event := &Event{
		Streamer: streamer,
		Outcomes: outcomes,
		Decision: Decision{Choice: 1},
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
	outcomes := []Outcome{
		{TotalUsers: 5, TotalPoints: 10},
		{TotalUsers: 50, TotalPoints: 20},
	}
	streamer := &domain.Streamer{
		Settings: domain.StreamerSettings{
			Bet: domain.BetSettings{
				FilterCondition: &domain.FilterCondition{
					By:    domain.OutcomeDecisionUsers,
					Where: domain.ConditionLT,
					Value: ptrFloat(40),
				},
			},
		},
	}
	event := &Event{
		Streamer: streamer,
		Outcomes: outcomes,
		Decision: Decision{Choice: 1},
	}

	skip, compared, _ := event.ShouldSkipByFilter()
	if !skip {
		t.Fatalf("expected skip when decision users 50 !< 40")
	}
	if compared != 50 {
		t.Fatalf("expected decision users compared 50, got %.0f", compared)
	}

	// ? Loosen threshold to allow betting
	event.Streamer.Settings.Bet.FilterCondition.Where = domain.ConditionGTE
	event.Streamer.Settings.Bet.FilterCondition.Value = ptrFloat(50)
	skip, compared, reason := event.ShouldSkipByFilter()
	if skip {
		t.Fatalf("expected bet allowed, got skip (compared %.0f, reason %s)", compared, reason)
	}
}
