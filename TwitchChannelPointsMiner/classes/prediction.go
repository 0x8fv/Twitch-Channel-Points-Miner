package classes

import (
	"math"
	"sort"
	"strings"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/classes/entities"
)

type PredictionOutcome struct {
	ID              string
	Title           string
	Color           string
	TotalUsers      int
	TotalPoints     int
	TopPoints       int
	PercentageUsers float64
	Odds            float64
	OddsPercentage  float64
}

type PredictionDecision struct {
	Choice    int
	OutcomeID string
	Amount    int
}

type PredictionEvent struct {
	Streamer      *entities.Streamer
	EventID       string
	Title         string
	Status        string
	CreatedAt     time.Time
	WindowSeconds float64
	Outcomes      []PredictionOutcome
	Decision      PredictionDecision
	BetPlaced     bool
	BetConfirmed  bool
	ResultType    string
}

func NewPredictionEvent(streamer *entities.Streamer, event map[string]interface{}) *PredictionEvent {
	if streamer == nil || event == nil {
		return nil
	}
	eventID, _ := event["id"].(string)
	title, _ := event["title"].(string)
	status := strings.ToUpper(stringOrDefault(event["status"]))
	window := float64(fromFloat(event["prediction_window_seconds"]))
	created := time.Now()
	if createdStr, ok := event["created_at"].(string); ok && createdStr != "" {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			created = t
		}
	}
	pe := &PredictionEvent{
		Streamer:      streamer,
		EventID:       eventID,
		Title:         strings.TrimSpace(title),
		Status:        status,
		CreatedAt:     created,
		WindowSeconds: window,
		BetPlaced:     false,
	}
	rawOutcomes, _ := event["outcomes"].([]interface{})
	pe.UpdateOutcomes(rawOutcomes)
	return pe
}

func (p *PredictionEvent) UpdateOutcomes(outcomes []interface{}) {
	parsed := make([]PredictionOutcome, 0, len(outcomes))
	totalUsers := 0
	totalPoints := 0
	for _, raw := range outcomes {
		oc, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		outcome := PredictionOutcome{
			ID:          stringOrDefault(oc["id"]),
			Title:       stringOrDefault(oc["title"]),
			Color:       stringOrDefault(oc["color"]),
			TotalUsers:  int(fromFloat(oc["total_users"])),
			TotalPoints: int(fromFloat(oc["total_points"])),
		}
		if topPredictors, ok := oc["top_predictors"].([]interface{}); ok && len(topPredictors) > 0 {
			if first, ok := topPredictors[0].(map[string]interface{}); ok {
				outcome.TopPoints = int(fromFloat(first["points"]))
			}
		}
		parsed = append(parsed, outcome)
		totalUsers += outcome.TotalUsers
		totalPoints += outcome.TotalPoints
	}
	for i := range parsed {
		if totalUsers > 0 {
			parsed[i].PercentageUsers = (float64(parsed[i].TotalUsers) * 100) / float64(totalUsers)
		}
		if parsed[i].TotalPoints > 0 {
			parsed[i].Odds = float64(totalPoints) / float64(parsed[i].TotalPoints)
		}
		if parsed[i].Odds > 0 {
			parsed[i].OddsPercentage = 100 / parsed[i].Odds
		}
	}
	p.Outcomes = parsed
}

func (p *PredictionEvent) ClosingAfter(now time.Time) time.Duration {
	elapsed := now.Sub(p.CreatedAt).Seconds()
	remaining := p.WindowSeconds - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return time.Duration(remaining * float64(time.Second))
}

func (p *PredictionEvent) Decide(balance int) PredictionDecision {
	decision := PredictionDecision{}
	if p.Streamer == nil || len(p.Outcomes) == 0 {
		return decision
	}
	settings := p.Streamer.Settings.Bet

	choice := selectOutcome(p.Outcomes, settings)
	if choice < 0 || choice >= len(p.Outcomes) {
		return decision
	}
	percentage := 5
	if settings.Percentage != nil {
		percentage = *settings.Percentage
	}
	amount := int(float64(balance) * (float64(percentage) / 100))
	if settings.MaxPoints != nil && amount > *settings.MaxPoints {
		amount = *settings.MaxPoints
	}
	if settings.StealthMode != nil && *settings.StealthMode && p.Outcomes[choice].TopPoints > 0 && amount >= p.Outcomes[choice].TopPoints {
		amount = p.Outcomes[choice].TopPoints - 1
		if amount < 1 {
			amount = 1
		}
	}

	decision = PredictionDecision{
		Choice:    choice,
		OutcomeID: p.Outcomes[choice].ID,
		Amount:    amount,
	}
	p.Decision = decision
	p.BetPlaced = amount > 0
	return decision
}

func (p *PredictionEvent) ParseResult(result map[string]interface{}) (gained, placed, won int, resultType string) {
	resultType = strings.ToUpper(stringOrDefault(result["type"]))
	placed = p.Decision.Amount
	won = int(fromFloat(result["points_won"]))
	if resultType == "REFUND" {
		placed = 0
		won = 0
	}
	gained = won - placed
	p.ResultType = resultType
	return
}

func selectOutcome(outcomes []PredictionOutcome, settings entities.BetSettings) int {
	if len(outcomes) == 0 {
		return -1
	}
	strategy := settings.Strategy
	if strategy == "" {
		strategy = entities.StrategySmart
	}

	switch strategy {
	case entities.StrategyMostVoted:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return float64(o.TotalUsers) })
	case entities.StrategyHighOdds:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.Odds })
	case entities.StrategyPercentage:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.OddsPercentage })
	case entities.StrategySmartMoney:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return float64(o.TopPoints) })
	case entities.StrategyNumber1:
		if len(outcomes) > 0 {
			return 0
		}
	case entities.StrategyNumber2:
		if len(outcomes) > 1 {
			return 1
		}
	case entities.StrategyNumber3:
		if len(outcomes) > 2 {
			return 2
		}
	case entities.StrategyNumber4:
		if len(outcomes) > 3 {
			return 3
		}
	case entities.StrategyNumber5:
		if len(outcomes) > 4 {
			return 4
		}
	case entities.StrategyNumber6:
		if len(outcomes) > 5 {
			return 5
		}
	case entities.StrategyNumber7:
		if len(outcomes) > 6 {
			return 6
		}
	case entities.StrategyNumber8:
		if len(outcomes) > 7 {
			return 7
		}
	case entities.StrategySmart:
		gap := 20
		if settings.PercentageGap != nil {
			gap = *settings.PercentageGap
		}
		percents := append([]PredictionOutcome(nil), outcomes...)
		sort.SliceStable(percents, func(i, j int) bool {
			return percents[i].PercentageUsers > percents[j].PercentageUsers
		})
		if len(percents) >= 2 {
			if math.Abs(percents[0].PercentageUsers-percents[1].PercentageUsers) < float64(gap) {
				return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.Odds })
			}
		}
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return float64(o.TotalUsers) })
	}

	return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.Odds })
}

func maxIndex(outcomes []PredictionOutcome, value func(PredictionOutcome) float64) int {
	if len(outcomes) == 0 {
		return -1
	}
	best := 0
	bestVal := value(outcomes[0])
	for i := 1; i < len(outcomes); i++ {
		if v := value(outcomes[i]); v > bestVal {
			best = i
			bestVal = v
		}
	}
	return best
}

func stringOrDefault(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
