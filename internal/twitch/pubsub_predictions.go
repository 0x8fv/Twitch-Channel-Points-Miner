package twitch

import (
	"fmt"
	"math"
	"strings"
	"time"

	"TwitchChannelPointsMiner/internal/ansi"
	"TwitchChannelPointsMiner/internal/events"
)

func (p *PubSubClient) processPredictionChannel(topic string, payload map[string]interface{}) error {
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
	status := strings.ToUpper(stringOrDefault(eventMap["status"]))

	msgType := strings.ToLower(fmt.Sprint(payload["type"]))

	switch msgType {
	case "event-created":
		if status != "ACTIVE" || eventID == "" {
			return nil
		}
		window := eventMap["prediction_window_seconds"]
		eventMap["prediction_window_seconds"] = streamer.PredictionWindowSeconds(fromFloat(window))
		event := NewPredictionEvent(streamer, eventMap)
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
		var existing *PredictionEvent
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

func (p *PubSubClient) processPredictionUser(payload map[string]interface{}) error {
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

func (p *PubSubClient) placePrediction(eventID string) {
	p.predMu.Lock()
	event, ok := p.predictions[eventID]
	p.predMu.Unlock()
	if !ok || event == nil || event.Streamer == nil {
		return
	}
	stopTracking := func(markResult string) {
		if event != nil && markResult != "" {
			event.ResultType = markResult
		}
		p.predMu.Lock()
		delete(p.predictions, eventID)
		p.predMu.Unlock()
	}
	streamer := event.Streamer
	if event.Status != "ACTIVE" {
		p.logger.Eventf(events.EventBetGeneral, "Skip bet for %s: event status is %s", p.streamerName(streamer), event.Status)
		stopTracking("SKIPPED")
		return
	}
	if streamer.Settings.Bet.MinimumPoints != nil && streamer.ChannelPoints <= *streamer.Settings.Bet.MinimumPoints {
		if p.anonymizeLogs() {
			p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: balance below minimum_points", p.streamerName(streamer))
		} else {
			p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: balance %d <= minimum_points %d", streamer.Username, streamer.ChannelPoints, *streamer.Settings.Bet.MinimumPoints)
		}
		stopTracking("SKIPPED")
		return
	}
	decision := event.Decide(streamer.ChannelPoints)
	if decision.OutcomeID == "" {
		p.logger.Eventf(events.EventBetGeneral, "Skip bet for %s: no outcome selected", p.streamerName(streamer))
		stopTracking("SKIPPED")
		return
	}
	if skip, compared, reason := event.ShouldSkipByFilter(); skip {
		if reason == "" {
			if p.anonymizeLogs() {
				reason = "filter_condition not satisfied"
			} else {
				reason = fmt.Sprintf("filter_condition not satisfied (current %s)", formatFloat(compared))
			}
		}
		if p.anonymizeLogs() {
			p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: %s", p.streamerName(streamer), reason)
			stopTracking("SKIPPED")
			return
		}
		p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: %s", streamer.Username, reason)
		stopTracking("SKIPPED")
		return
	}
	if decision.Amount < 10 {
		reason := fmt.Sprintf("balance %d below Twitch minimum 10", streamer.ChannelPoints)
		if streamer.ChannelPoints >= 10 {
			if streamer.Settings.Bet.MaxPoints != nil && *streamer.Settings.Bet.MaxPoints < 10 {
				reason = fmt.Sprintf("max_points %d below Twitch minimum 10", *streamer.Settings.Bet.MaxPoints)
			} else {
				reason = fmt.Sprintf("calculated stake %d below Twitch minimum", decision.Amount)
			}
		}
		if p.anonymizeLogs() {
			p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: below Twitch minimum", p.streamerName(streamer))
		} else {
			p.logger.Eventf(events.EventBetFilters, "Skip bet for %s: %s", streamer.Username, reason)
		}
		stopTracking("SKIPPED")
		return
	}
	if err := p.twitch.MakePrediction(event); err != nil {
		p.logger.ErrorEventf(events.EventBetFailed, "prediction %s: %v", p.streamerName(streamer), err)
		stopTracking("ERROR")
		return
	}

	// ? The payout/refund is reflected later via PubSub points-earned events (and/or API refresh)
	deductStake := true
	if streamer.Settings.Bet.DeductStakeOnPlace != nil {
		deductStake = *streamer.Settings.Bet.DeductStakeOnPlace
	}
	if deductStake && decision.Amount > 0 {
		if p.onGain != nil {
			p.onGain(streamer, -decision.Amount, "PREDICTION", 0)
		} else {
			streamer.ChannelPoints -= decision.Amount
			if streamer.ChannelPoints < 0 {
				streamer.ChannelPoints = 0
			}
		}
	}
	event.BetPlaced = true
	// ? Ensure we log results even if Twitch doesn't emit prediction-made
	event.BetConfirmed = true
	outcome := event.DecisionOutcomeString()
	if outcome == "" {
		outcome = decision.OutcomeID
	}
	if p.anonymizeLogs() {
		p.logger.EmojiEventf(":four_leaf_clover:", events.EventBetGeneral, "Place bet on: %s for %s", outcome, p.streamerName(streamer))
	} else {
		p.logger.EmojiEventf(":four_leaf_clover:", events.EventBetGeneral, "Place %s points on: %s for %s", formatNumber(decision.Amount), outcome, streamer.Username)
	}
}

func (p *PubSubClient) logPredictionResult(event *PredictionEvent, result map[string]interface{}) {
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

func (p *PubSubClient) resolvePredictionFromChannel(event *PredictionEvent, eventMap map[string]interface{}) {
	if event == nil || !event.BetPlaced || event.Decision.Amount == 0 || event.ResultType != "" {
		return
	}
	status := strings.ToUpper(stringOrDefault(eventMap["status"]))
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
	if id := stringOrDefault(event["winning_outcome_id"]); id != "" {
		return id
	}
	if outcomes, ok := event["outcomes"].([]interface{}); ok {
		for _, raw := range outcomes {
			oc, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if win, ok := oc["is_winning_outcome"].(bool); ok && win {
				if id := stringOrDefault(oc["id"]); id != "" {
					return id
				}
			}
			state := strings.ToUpper(stringOrDefault(oc["state"]))
			if state == "RESOLVED" || state == "WINNER" || state == "WIN" {
				if id := stringOrDefault(oc["id"]); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

func payoutForOutcome(decision PredictionDecision, outcomes []PredictionOutcome, winningID string) int {
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
