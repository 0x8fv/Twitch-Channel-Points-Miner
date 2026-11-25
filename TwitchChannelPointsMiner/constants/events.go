package constants

import "strings"

type Event string

const (
	EventStartup            Event = "STARTUP"
	EventShutdown           Event = "SHUTDOWN"
	EventStreamerOnline     Event = "STREAMER_ONLINE"
	EventStreamerOffline    Event = "STREAMER_OFFLINE"
	EventGainForRaid        Event = "GAIN_FOR_RAID"
	EventGainForClaim       Event = "GAIN_FOR_CLAIM"
	EventGainForWatch       Event = "GAIN_FOR_WATCH"
	EventGainForWatchStreak Event = "GAIN_FOR_WATCH_STREAK"
	EventBetWin             Event = "BET_WIN"
	EventBetLose            Event = "BET_LOSE"
	EventBetRefund          Event = "BET_REFUND"
	EventBetFilters         Event = "BET_FILTERS"
	EventBetGeneral         Event = "BET_GENERAL"
	EventBetFailed          Event = "BET_FAILED"
	EventBetStart           Event = "BET_START"
	EventBonusClaim         Event = "BONUS_CLAIM"
	EventMomentClaim        Event = "MOMENT_CLAIM"
	EventJoinRaid           Event = "JOIN_RAID"
	EventDropClaim          Event = "DROP_CLAIM"
	EventDropStatus         Event = "DROP_STATUS"
	EventChatMention        Event = "CHAT_MENTION"
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

// Tone is the polarity of an event: a gain or a live state, a loss or a drop, or neither.
type Tone string

const (
	TonePositive Tone = "pos"
	ToneNegative Tone = "neg"
	ToneNeutral  Tone = "neu"
)

type eventGlyph struct {
	emoji string
	tone  Tone
}

// One mark per event. Faces and celebration icons are intentionally absent:
// the glyph names the type, the tone says whether it helped or hurt.
var eventGlyphs = map[Event]eventGlyph{
	EventStartup:            {"▶️", TonePositive},
	EventShutdown:           {"⏹️", ToneNeutral},
	EventStreamerOnline:     {"🟢", TonePositive},
	EventStreamerOffline:    {"🔴", ToneNegative},
	EventGainForWatch:       {"➕", TonePositive},
	EventGainForWatchStreak: {"⏫", TonePositive},
	EventGainForClaim:       {"✅", TonePositive},
	EventBonusClaim:         {"✅", TonePositive},
	EventGainForRaid:        {"📡", TonePositive},
	EventJoinRaid:           {"↪️", TonePositive},
	EventBetWin:             {"📈", TonePositive},
	EventBetLose:            {"📉", ToneNegative},
	EventBetRefund:          {"↩️", ToneNeutral},
	EventBetStart:           {"⏳", ToneNeutral},
	EventBetGeneral:         {"🎯", ToneNeutral},
	EventBetFilters:         {"⏭️", ToneNeutral},
	EventBetFailed:          {"⛔", ToneNegative},
	EventDropClaim:          {"📦", TonePositive},
	EventDropStatus:         {"📋", ToneNeutral},
	EventMomentClaim:        {"⚡", TonePositive},
	EventChatMention:        {"💬", ToneNeutral},
}

// EventGlyph returns the emoji and polarity for a known event.
func EventGlyph(event Event) (emoji string, tone Tone, ok bool) {
	g, ok := eventGlyphs[event]
	if !ok {
		return "", "", false
	}
	return g.emoji, g.tone, true
}
