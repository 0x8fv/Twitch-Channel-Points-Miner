package entities

import "time"

type FollowersOrder string

const (
	FollowersOrderASC  FollowersOrder = "ASC"
	FollowersOrderDESC FollowersOrder = "DESC"
)

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

type BetSettings struct {
	Strategy        Strategy  `json:"strategy,omitempty"`
	Percentage      *int      `json:"percentage,omitempty"`
	PercentageGap   *int      `json:"percentage_gap,omitempty"`
	MaxPoints       *int      `json:"max_points,omitempty"`
	MinimumPoints   *int      `json:"minimum_points,omitempty"`
	StealthMode     *bool     `json:"stealth_mode,omitempty"`
	FilterCondition *string   `json:"filter_condition,omitempty"`
	Delay           *float64  `json:"delay,omitempty"`
	DelayMode       DelayMode `json:"delay_mode,omitempty"`
}

type StreamerSettings struct {
	MakePredictions bool        `json:"make_predictions"`
	FollowRaid      bool        `json:"follow_raid"`
	ClaimDrops      bool        `json:"claim_drops"`
	ClaimMoments    bool        `json:"claim_moments"`
	WatchStreak     bool        `json:"watch_streak"`
	CommunityGoals  bool        `json:"community_goals"`
	Bet             BetSettings `json:"bet"`
}

type Streamer struct {
	Username          string                   `json:"username"`
	ChannelID         string                   `json:"channel_id"`
	ChannelPoints     int                      `json:"channel_points"`
	Settings          StreamerSettings         `json:"settings"`
	StreamerURL       string                   `json:"-"`
	IsOnline          bool                     `json:"-"`
	PresenceKnown     bool                     `json:"-"`
	OnlineAt          time.Time                `json:"-"`
	OfflineAt         time.Time                `json:"-"`
	Stream            *Stream                  `json:"-"`
	PointsInit        bool                     `json:"-"`
	ActiveMultipliers []map[string]interface{} `json:"-"`
	LastRaidID        string                   `json:"-"`
	History           map[string]*HistoryEntry
	CommunityGoals    map[string]*CommunityGoal `json:"-"`
}

type HistoryEntry struct {
	Count  int
	Amount int
}

func (s *Streamer) HasActiveMultipliers() bool {
	return len(s.ActiveMultipliers) > 0
}

func (s *Streamer) TotalMultiplier() float64 {
	total := 0.0
	for _, mult := range s.ActiveMultipliers {
		if mult == nil {
			continue
		}
		switch v := mult["factor"].(type) {
		case float64:
			total += v
		case int:
			total += float64(v)
		}
	}
	return total
}

func (s *Streamer) PredictionWindowSeconds(predictionWindow float64) float64 {
	delay := 0.0
	if s.Settings.Bet.Delay != nil {
		delay = *s.Settings.Bet.Delay
	}
	switch s.Settings.Bet.DelayMode {
	case DelayModeFromStart:
		if delay < predictionWindow {
			return delay
		}
		return predictionWindow
	case DelayModeFromEnd:
		if predictionWindow-delay > 0 {
			return predictionWindow - delay
		}
		return 0
	case DelayModePercentage:
		return predictionWindow * delay
	default:
		return predictionWindow
	}
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
	if b.DelayMode == "" {
		b.DelayMode = DelayModeFromEnd
	}
	if b.Delay == nil {
		d := 6.0
		b.Delay = &d
	}
}

func (s *StreamerSettings) Default() {
	s.Bet.Default()
}
