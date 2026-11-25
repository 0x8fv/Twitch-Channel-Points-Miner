package twitch

import (
	"fmt"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/domain"
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
	Streamer      *domain.Streamer
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
	ResultString  string
}

func NewPredictionEvent(streamer *domain.Streamer, event map[string]interface{}) *PredictionEvent {
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

func (p *PredictionEvent) ParseResult(result map[string]interface{}) (gained, placed, won int, resultType, resultString string) {
	resultType = strings.ToUpper(stringOrDefault(result["type"]))
	placed = p.Decision.Amount
	won = int(fromFloat(result["points_won"]))
	if resultType == "REFUND" {
		placed = 0
		won = 0
	}
	gained = won - placed
	p.ResultType = resultType
	action := "Gained"
	switch resultType {
	case "LOSE":
		action = "Lost"
	case "REFUND":
		action = "Refunded"
	}
	sign := ""
	if gained >= 0 {
		sign = "+"
	}
	resultString = fmt.Sprintf("%s, %s: %s%s", resultType, action, sign, formatNumber(gained))
	p.ResultString = resultString
	return
}

func (p *PredictionEvent) String() string {
	if p.Streamer != nil && p.Streamer.Username != "" {
		return fmt.Sprintf("EventPrediction: %s - %s", p.Streamer.Username, p.Title)
	}
	if p.Title != "" {
		return fmt.Sprintf("EventPrediction: %s", p.Title)
	}
	return "EventPrediction"
}

func (p *PredictionEvent) DecisionOutcome() *PredictionOutcome {
	choice := p.Decision.Choice
	if choice >= 0 && choice < len(p.Outcomes) {
		return &p.Outcomes[choice]
	}
	for i := range p.Outcomes {
		if p.Outcomes[i].ID == p.Decision.OutcomeID {
			return &p.Outcomes[i]
		}
	}
	return nil
}

func (p *PredictionEvent) DecisionOutcomeString() string {
	if out := p.DecisionOutcome(); out != nil {
		return out.String()
	}
	return p.Decision.OutcomeID
}

func (p *PredictionEvent) DecisionLabel() string {
	out := p.DecisionOutcome()
	if out == nil {
		if p.Decision.OutcomeID != "" {
			return fmt.Sprintf("%s: %s", choiceLabel(p.Decision.Choice), p.Decision.OutcomeID)
		}
		return ""
	}
	return fmt.Sprintf("%s: %s (%s)", choiceLabel(p.Decision.Choice), out.Title, strings.ToUpper(out.Color))
}

func (o PredictionOutcome) String() string {
	return fmt.Sprintf(
		"%s (%s), Points: %s, Users: %s (%.2f%%), Odds: %s (%s%%)",
		strings.TrimSpace(o.Title),
		strings.ToUpper(o.Color),
		formatNumber(o.TotalPoints),
		formatNumber(o.TotalUsers),
		o.PercentageUsers,
		formatFloat(o.Odds),
		formatFloat(o.OddsPercentage),
	)
}

func choiceLabel(choice int) string {
	if choice >= 0 && choice < 26 {
		return string(rune('A' + choice))
	}
	return fmt.Sprintf("#%d", choice+1)
}
