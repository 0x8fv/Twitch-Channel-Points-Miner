package prediction

import (
	"fmt"
	"strings"

	"TwitchChannelPointsMiner/internal/domain"
)

// ? ShouldSkipByFilter evaluates the optional filter_condition before betting
// ? Returns true when the bet should be skipped, along with the compared value and a "human-friendly" reason
func (p *Event) ShouldSkipByFilter() (bool, float64, string) {
	if p == nil || p.Streamer == nil {
		return false, 0, ""
	}
	fc := p.Streamer.Settings.Bet.FilterCondition
	if fc == nil || fc.Value == nil || fc.By == "" || fc.Where == "" {
		return false, 0, ""
	}

	by := domain.OutcomeKey(strings.ToUpper(string(fc.By)))
	where := domain.Condition(strings.ToUpper(string(fc.Where)))
	value := *fc.Value

	valueByChoice := func(selector func(Outcome) float64) (float64, bool) {
		if p.Decision.Choice >= 0 && p.Decision.Choice < len(p.Outcomes) {
			return selector(p.Outcomes[p.Decision.Choice]), true
		}
		return 0, false
	}

	var compared float64
	switch by {
	case domain.OutcomeTotalUsers:
		for _, out := range p.Outcomes {
			compared += float64(out.TotalUsers)
		}
	case domain.OutcomeTotalPoints:
		for _, out := range p.Outcomes {
			compared += float64(out.TotalPoints)
		}
	case domain.OutcomeDecisionUsers:
		if v, ok := valueByChoice(func(o Outcome) float64 { return float64(o.TotalUsers) }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	case domain.OutcomeDecisionPoints:
		if v, ok := valueByChoice(func(o Outcome) float64 { return float64(o.TotalPoints) }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	case domain.OutcomePercentageUsers:
		if v, ok := valueByChoice(func(o Outcome) float64 { return o.PercentageUsers }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	case domain.OutcomeOdds:
		if v, ok := valueByChoice(func(o Outcome) float64 { return o.Odds }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	case domain.OutcomeOddsPercentage:
		if v, ok := valueByChoice(func(o Outcome) float64 { return o.OddsPercentage }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	case domain.OutcomeTopPoints:
		if v, ok := valueByChoice(func(o Outcome) float64 { return float64(o.TopPoints) }); ok {
			compared = v
		} else {
			return true, 0, "filter_condition requires a decision outcome"
		}
	default:
		return true, 0, fmt.Sprintf("filter_condition 'by' unsupported: %s", by)
	}

	var pass bool
	switch where {
	case domain.ConditionGT:
		pass = compared > value
	case domain.ConditionGTE:
		pass = compared >= value
	case domain.ConditionLT:
		pass = compared < value
	case domain.ConditionLTE:
		pass = compared <= value
	default:
		return true, compared, fmt.Sprintf("filter_condition 'where' unsupported: %s", where)
	}

	if pass {
		return false, compared, ""
	}
	return true, compared, fmt.Sprintf("filter_condition %s %s %s not met (current %s)", by, where, FormatFloat(value), FormatFloat(compared))
}
