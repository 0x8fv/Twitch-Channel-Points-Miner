package twitch

import (
	"math"
	"sort"

	"TwitchChannelPointsMiner/internal/domain"
)

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
	if amount > balance {
		amount = balance
	}
	if settings.StealthMode != nil && *settings.StealthMode && p.Outcomes[choice].TopPoints > 0 && amount >= p.Outcomes[choice].TopPoints {
		amount = p.Outcomes[choice].TopPoints - 1
		if amount < 1 {
			amount = 1
		}
	}
	if amount < 10 {
		if settings.MaxPoints != nil && *settings.MaxPoints < 10 {
			amount = *settings.MaxPoints
		} else if balance >= 10 {
			amount = 10
		}
	}

	decision = PredictionDecision{
		Choice:    choice,
		OutcomeID: p.Outcomes[choice].ID,
		Amount:    amount,
	}
	p.Decision = decision
	return decision
}

func selectOutcome(outcomes []PredictionOutcome, settings domain.BetSettings) int {
	if len(outcomes) == 0 {
		return -1
	}
	strategy := settings.Strategy
	if strategy == "" {
		strategy = domain.StrategySmart
	}

	switch strategy {
	case domain.StrategyMostVoted:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return float64(o.TotalUsers) })
	case domain.StrategyHighOdds:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.Odds })
	case domain.StrategyPercentage:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return o.OddsPercentage })
	case domain.StrategySmartMoney:
		return maxIndex(outcomes, func(o PredictionOutcome) float64 { return float64(o.TopPoints) })
	case domain.StrategyNumber1:
		if len(outcomes) > 0 {
			return 0
		}
	case domain.StrategyNumber2:
		if len(outcomes) > 1 {
			return 1
		}
	case domain.StrategyNumber3:
		if len(outcomes) > 2 {
			return 2
		}
	case domain.StrategyNumber4:
		if len(outcomes) > 3 {
			return 3
		}
	case domain.StrategyNumber5:
		if len(outcomes) > 4 {
			return 4
		}
	case domain.StrategyNumber6:
		if len(outcomes) > 5 {
			return 5
		}
	case domain.StrategyNumber7:
		if len(outcomes) > 6 {
			return 6
		}
	case domain.StrategyNumber8:
		if len(outcomes) > 7 {
			return 7
		}
	case domain.StrategySmart:
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
