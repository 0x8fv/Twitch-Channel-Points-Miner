package pubsub

import (
	"fmt"

	"TwitchChannelPointsMiner/internal/events"
	"TwitchChannelPointsMiner/internal/prediction"
)

func (p *Client) placePrediction(eventID string) {
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
				reason = fmt.Sprintf("filter_condition not satisfied (current %s)", prediction.FormatFloat(compared))
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
		p.logger.EmojiEventf(":four_leaf_clover:", events.EventBetGeneral, "Place %s points on: %s for %s", prediction.FormatNumber(decision.Amount), outcome, streamer.Username)
	}
}
