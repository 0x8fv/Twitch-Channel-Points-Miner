package twitch

import (
	"fmt"

	"TwitchChannelPointsMiner/internal/domain"
)

// ? ContributeToCommunityGoals mirrors the site behavior by spending points into active community goals.
func (t *Twitch) ContributeToCommunityGoals(streamer *domain.Streamer) {
	if streamer == nil || !streamer.Settings.CommunityGoals || len(streamer.CommunityGoals) == 0 {
		return
	}
	hasActive := false
	for _, goal := range streamer.CommunityGoals {
		if goal != nil && goal.Status == "STARTED" && goal.IsInStock {
			hasActive = true
			break
		}
	}
	if !hasActive {
		return
	}

	op := ClonePersistedOperation(GQLOperations.UserPointsContribution)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["channelLogin"] = streamer.Username
	resp, err := t.PostGQL(op)
	if err != nil {
		return
	}
	contribs := navigate(resp, "data.user.channel.self.communityPoints.goalContributions")
	arr, ok := contribs.([]interface{})
	if !ok {
		return
	}
	for _, raw := range arr {
		goalContribution, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		goalData, _ := goalContribution["goal"].(map[string]interface{})
		goalID, _ := goalData["id"].(string)
		if goalID == "" {
			continue
		}
		goal := streamer.CommunityGoals[goalID]
		if goal == nil {
			continue
		}
		userPoints := int(fromFloat(goalContribution["userPointsContributedThisStream"]))
		userLeft := goal.PerStreamUserMaximumContribution - userPoints
		amount := minInt(goal.AmountLeft(), userLeft, streamer.ChannelPoints)
		if amount > 0 {
			_ = t.ContributeToCommunityGoal(streamer, goalID, goal.Title, amount)
		}
	}
}

// ? ContributeToCommunityGoal sends a single contribution transaction.
func (t *Twitch) ContributeToCommunityGoal(streamer *domain.Streamer, goalID, title string, amount int) error {
	if amount <= 0 || goalID == "" {
		return nil
	}
	op := ClonePersistedOperation(GQLOperations.ContributeCommunityPointsCommunityGoal)
	if op.Variables == nil {
		op.Variables = map[string]interface{}{}
	}
	op.Variables["input"] = map[string]interface{}{
		"amount":        amount,
		"channelID":     streamer.ChannelID,
		"goalID":        goalID,
		"transactionID": randomHex(16),
	}
	resp, err := t.PostGQL(op)
	if err != nil {
		return err
	}
	if errVal := navigate(resp, "data.contributeCommunityPointsCommunityGoal.error"); errVal != nil {
		if errStr, ok := errVal.(string); ok && errStr != "" {
			return fmt.Errorf("unable to contribute to %s: %s", title, errStr)
		}
	}
	streamer.ChannelPoints -= amount
	if streamer.ChannelPoints < 0 {
		streamer.ChannelPoints = 0
	}
	return nil
}

func parseCommunityGoals(goals interface{}) map[string]*domain.CommunityGoal {
	arr, ok := goals.([]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]*domain.CommunityGoal, len(arr))
	for _, raw := range arr {
		if goalMap, ok := raw.(map[string]interface{}); ok {
			if goal := domain.NewCommunityGoalFromGQL(goalMap); goal != nil && goal.ID != "" {
				result[goal.ID] = goal
			}
		}
	}
	return result
}
