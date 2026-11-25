package pubsub

import (
	"fmt"
	"math"
	"strings"

	"TwitchChannelPointsMiner/internal/ansi"
	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/jsonvalue"
	"TwitchChannelPointsMiner/internal/prediction"
)

func (p *Client) logPredictionResult(event *prediction.Event, result map[string]interface{}) {
	if event == nil || result == nil {
		return
	}
	gained, _, _, resultType, resultString := event.ParseResult(result)
	event.BetConfirmed = true
	decisionLabel := event.DecisionLabel()
	if decisionLabel == "" {
		decisionLabel = fmt.Sprintf("Decision #%d", event.Decision.Choice+1)
	}
	color := ansi.ColorGreen
	if gained < 0 {
		color = ansi.ColorRed
	} else if gained == 0 {
		color = ansi.ColorReset
	}
	label := event.String()
	if p.anonymizeLogs() {
		name := ""
		if event.Streamer != nil {
			name = p.streamerName(event.Streamer)
		}
		if name != "" && event.Title != "" {
			label = fmt.Sprintf("EventPrediction: %s - %s", name, event.Title)
		} else if name != "" {
			label = fmt.Sprintf("EventPrediction: %s", name)
		} else if event.Title != "" {
			label = fmt.Sprintf("EventPrediction: %s", event.Title)
		} else {
			label = "EventPrediction"
		}
		resultString = resultType
	}
	resultEvent := events.EventFromBetResult(resultType)
	if resultEvent != "" {
		p.logger.EmojiEventf(
			":bar_chart:",
			resultEvent,
			"%s - Decision: %s - Result: %s%s%s",
			label,
			decisionLabel,
			color,
			resultString,
			ansi.ColorReset,
		)
	} else {
		p.logger.EmojiPrintf(
			":bar_chart:",
			"%s - Decision: %s - Result: %s%s%s",
			label,
			decisionLabel,
			color,
			resultString,
			ansi.ColorReset,
		)
	}
}

func (p *Client) resolvePredictionFromChannel(event *prediction.Event, eventMap map[string]interface{}) {
	if event == nil || !event.BetPlaced || event.Decision.Amount == 0 || event.ResultType != "" {
		return
	}
	status := strings.ToUpper(jsonvalue.String(eventMap["status"]))
	if status != "RESOLVED" && status != "CANCELED" && status != "CANCELLED" {
		return
	}

	winningID := winningOutcomeID(eventMap)
	resultType := "LOSE"
	pointsWon := 0

	switch status {
	case "CANCELED", "CANCELLED":
		resultType = "REFUND"
	default:
		if winningID == "" {
			return
		}
		if event.Decision.OutcomeID == winningID {
			resultType = "WIN"
			pointsWon = payoutForOutcome(event.Decision, event.Outcomes, winningID)
		}
	}

	p.logPredictionResult(event, map[string]interface{}{
		"type":       resultType,
		"points_won": pointsWon,
	})

	p.predMu.Lock()
	delete(p.predictions, event.EventID)
	p.predMu.Unlock()
}

func winningOutcomeID(event map[string]interface{}) string {
	if id := jsonvalue.String(event["winning_outcome_id"]); id != "" {
		return id
	}
	if outcomes, ok := event["outcomes"].([]interface{}); ok {
		for _, raw := range outcomes {
			oc, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if win, ok := oc["is_winning_outcome"].(bool); ok && win {
				if id := jsonvalue.String(oc["id"]); id != "" {
					return id
				}
			}
			state := strings.ToUpper(jsonvalue.String(oc["state"]))
			if state == "RESOLVED" || state == "WINNER" || state == "WIN" {
				if id := jsonvalue.String(oc["id"]); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

func payoutForOutcome(decision prediction.Decision, outcomes []prediction.Outcome, winningID string) int {
	if decision.Amount <= 0 || decision.OutcomeID != winningID {
		return 0
	}

	totalPoints := 0
	winPoints := 0
	for _, oc := range outcomes {
		totalPoints += oc.TotalPoints
		if oc.ID == winningID {
			winPoints = oc.TotalPoints
		}
	}

	if totalPoints == 0 || winPoints == 0 {
		return decision.Amount
	}

	payout := int(math.Round(float64(decision.Amount) * (float64(totalPoints) / float64(winPoints))))
	if payout < decision.Amount {
		return decision.Amount
	}
	return payout
}
