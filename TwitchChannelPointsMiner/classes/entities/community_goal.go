package entities

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
		ID:                               stringOrDefault(goal["id"]),
		Title:                            stringOrDefault(goal["title"]),
		IsInStock:                        boolOrDefault(goal["isInStock"]),
		PointsContributed:                intFromAny(goal["pointsContributed"]),
		AmountNeeded:                     intFromAny(goal["amountNeeded"]),
		PerStreamUserMaximumContribution: intFromAny(goal["perStreamUserMaximumContribution"]),
		Status:                           stringOrDefault(goal["status"]),
	}
}

func NewCommunityGoalFromPubSub(goal map[string]interface{}) *CommunityGoal {
	if goal == nil {
		return nil
	}
	return &CommunityGoal{
		ID:                               stringOrDefault(goal["id"]),
		Title:                            stringOrDefault(goal["title"]),
		IsInStock:                        boolOrDefault(goal["is_in_stock"]),
		PointsContributed:                intFromAny(goal["points_contributed"]),
		AmountNeeded:                     intFromAny(goal["goal_amount"]),
		PerStreamUserMaximumContribution: intFromAny(goal["per_stream_maximum_user_contribution"]),
		Status:                           stringOrDefault(goal["status"]),
	}
}

func stringOrDefault(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolOrDefault(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
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
