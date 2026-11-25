package domain

type FollowersOrder string

const (
	FollowersOrderASC  FollowersOrder = "ASC"
	FollowersOrderDESC FollowersOrder = "DESC"
)

type IRCMode string

const (
	IRCModeAlways  IRCMode = "ALWAYS"
	IRCModeNever   IRCMode = "NEVER"
	IRCModeOnline  IRCMode = "ONLINE"
	IRCModeOffline IRCMode = "OFFLINE"
)

type StreamerSettings struct {
	MakePredictions bool        `json:"make_predictions"`
	FollowRaid      bool        `json:"follow_raid"`
	ClaimDrops      bool        `json:"claim_drops"`
	ClaimMoments    bool        `json:"claim_moments"`
	WatchStreak     bool        `json:"watch_streak"`
	CommunityGoals  bool        `json:"community_goals"`
	Bet             BetSettings `json:"bet"`
	IRCMode         IRCMode     `json:"chat_presence"`
}

func (s *StreamerSettings) Default() {
	s.Bet.Default()
	if s.IRCMode == "" {
		s.IRCMode = IRCModeOnline
	}
}
