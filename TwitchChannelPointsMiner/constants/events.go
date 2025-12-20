package constants

import "strings"

type Event string

const (
	EventStreamerOnline   Event = "STREAMER_ONLINE"
	EventStreamerOffline  Event = "STREAMER_OFFLINE"
	EventGainForRaid      Event = "GAIN_FOR_RAID"
	EventGainForClaim     Event = "GAIN_FOR_CLAIM"
	EventGainForWatch     Event = "GAIN_FOR_WATCH"
	EventGainForWatchStreak Event = "GAIN_FOR_WATCH_STREAK"
	EventBetWin           Event = "BET_WIN"
	EventBetLose          Event = "BET_LOSE"
	EventBetRefund        Event = "BET_REFUND"
	EventBetFilters       Event = "BET_FILTERS"
	EventBetGeneral       Event = "BET_GENERAL"
	EventBetFailed        Event = "BET_FAILED"
	EventBetStart         Event = "BET_START"
	EventBonusClaim       Event = "BONUS_CLAIM"
	EventMomentClaim      Event = "MOMENT_CLAIM"
	EventJoinRaid         Event = "JOIN_RAID"
	EventDropClaim        Event = "DROP_CLAIM"
	EventDropStatus       Event = "DROP_STATUS"
	EventChatMention      Event = "CHAT_MENTION"
)

func NormalizeEventName(raw string) Event {
	name := strings.ToUpper(strings.TrimSpace(raw))
	if name == "" {
		return ""
	}
	return Event(name)
}

func EventFromGainReason(reason string) Event {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "WATCH":
		return EventGainForWatch
	case "WATCH_STREAK":
		return EventGainForWatchStreak
	case "CLAIM":
		return EventGainForClaim
	case "RAID":
		return EventGainForRaid
	default:
		return ""
	}
}

func EventFromBetResult(result string) Event {
	switch strings.ToUpper(strings.TrimSpace(result)) {
	case "WIN":
		return EventBetWin
	case "LOSE":
		return EventBetLose
	case "REFUND":
		return EventBetRefund
	default:
		return ""
	}
}
