package domain

type Strategy string

const (
	StrategyMostVoted  Strategy = "MOST_VOTED"
	StrategyHighOdds   Strategy = "HIGH_ODDS"
	StrategyPercentage Strategy = "PERCENTAGE"
	StrategySmartMoney Strategy = "SMART_MONEY"
	StrategySmart      Strategy = "SMART"
	StrategyNumber1    Strategy = "NUMBER_1"
	StrategyNumber2    Strategy = "NUMBER_2"
	StrategyNumber3    Strategy = "NUMBER_3"
	StrategyNumber4    Strategy = "NUMBER_4"
	StrategyNumber5    Strategy = "NUMBER_5"
	StrategyNumber6    Strategy = "NUMBER_6"
	StrategyNumber7    Strategy = "NUMBER_7"
	StrategyNumber8    Strategy = "NUMBER_8"
)

type DelayMode string

const (
	DelayModeFromStart  DelayMode = "FROM_START"
	DelayModeFromEnd    DelayMode = "FROM_END"
	DelayModePercentage DelayMode = "PERCENTAGE"
)

type Condition string

const (
	ConditionGT  Condition = "GT"
	ConditionLT  Condition = "LT"
	ConditionGTE Condition = "GTE"
	ConditionLTE Condition = "LTE"
)

type OutcomeKey string

const (
	OutcomePercentageUsers OutcomeKey = "PERCENTAGE_USERS"
	OutcomeOdds            OutcomeKey = "ODDS"
	OutcomeOddsPercentage  OutcomeKey = "ODDS_PERCENTAGE"
	OutcomeTopPoints       OutcomeKey = "TOP_POINTS"
	OutcomeTotalUsers      OutcomeKey = "TOTAL_USERS"
	OutcomeTotalPoints     OutcomeKey = "TOTAL_POINTS"
	OutcomeDecisionUsers   OutcomeKey = "DECISION_USERS"
	OutcomeDecisionPoints  OutcomeKey = "DECISION_POINTS"
)

type FilterCondition struct {
	By    OutcomeKey `json:"by"`
	Where Condition  `json:"where"`
	Value *float64   `json:"value"`
}

type BetSettings struct {
	Strategy           Strategy         `json:"strategy,omitempty"`
	Percentage         *int             `json:"percentage,omitempty"`
	PercentageGap      *int             `json:"percentage_gap,omitempty"`
	MaxPoints          *int             `json:"max_points,omitempty"`
	MinimumPoints      *int             `json:"minimum_points,omitempty"`
	StealthMode        *bool            `json:"stealth_mode,omitempty"`
	DeductStakeOnPlace *bool            `json:"deduct_stake_on_place,omitempty"`
	FilterCondition    *FilterCondition `json:"filter_condition,omitempty"`
	Delay              *float64         `json:"delay,omitempty"`
	DelayMode          DelayMode        `json:"delay_mode,omitempty"`
}

func (b *BetSettings) Default() {
	if b.Strategy == "" {
		b.Strategy = StrategySmart
	}
	if b.Percentage == nil {
		v := 5
		b.Percentage = &v
	}
	if b.PercentageGap == nil {
		v := 20
		b.PercentageGap = &v
	}
	if b.MaxPoints == nil {
		v := 50000
		b.MaxPoints = &v
	}
	if b.MinimumPoints == nil {
		v := 0
		b.MinimumPoints = &v
	}
	if b.StealthMode == nil {
		v := false
		b.StealthMode = &v
	}
	if b.DeductStakeOnPlace == nil {
		v := true
		b.DeductStakeOnPlace = &v
	}
	if b.DelayMode == "" {
		b.DelayMode = DelayModeFromEnd
	}
	if b.Delay == nil {
		d := 6.0
		b.Delay = &d
	}
}
