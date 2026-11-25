package domain

import (
	"TwitchChannelPointsMiner/internal/jsonvalue"
)

type CommunityGoal struct {
	ID                               string
	Title                            string
	IsInStock                        bool
	PointsContributed                int
	AmountNeeded                     int
	PerStreamUserMaximumContribution int
	Status                           string
}

func (c *CommunityGoal) AmountLeft() int {
	return c.AmountNeeded - c.PointsContributed
}

func NewCommunityGoalFromGQL(goal map[string]interface{}) *CommunityGoal {
	if goal == nil {
		return nil
	}
	return &CommunityGoal{
		ID:                               jsonvalue.String(goal["id"]),
		Title:                            jsonvalue.String(goal["title"]),
		IsInStock:                        jsonvalue.Bool(goal["isInStock"]),
		PointsContributed:                intFromAny(goal["pointsContributed"]),
		AmountNeeded:                     intFromAny(goal["amountNeeded"]),
		PerStreamUserMaximumContribution: intFromAny(goal["perStreamUserMaximumContribution"]),
		Status:                           jsonvalue.String(goal["status"]),
	}
}

func NewCommunityGoalFromPubSub(goal map[string]interface{}) *CommunityGoal {
	if goal == nil {
		return nil
	}
	return &CommunityGoal{
		ID:                               jsonvalue.String(goal["id"]),
		Title:                            jsonvalue.String(goal["title"]),
		IsInStock:                        jsonvalue.Bool(goal["is_in_stock"]),
		PointsContributed:                intFromAny(goal["points_contributed"]),
		AmountNeeded:                     intFromAny(goal["goal_amount"]),
		PerStreamUserMaximumContribution: intFromAny(goal["per_stream_maximum_user_contribution"]),
		Status:                           jsonvalue.String(goal["status"]),
	}
}

func intFromAny(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
