package notify

import (
	"TwitchChannelPointsMiner/internal/events"
)

func eventTitle(event events.Event) string {
	switch event {
	case events.EventStartup:
		return "Session started"
	case events.EventShutdown:
		return "Session ended"
	case events.EventStreamerOnline:
		return "Online"
	case events.EventStreamerOffline:
		return "Offline"
	case events.EventGainForRaid:
		return "Raid points"
	case events.EventGainForClaim, events.EventBonusClaim:
		return "Bonus claimed"
	case events.EventGainForWatch:
		return "Watch"
	case events.EventGainForWatchStreak:
		return "Watch streak"
	case events.EventBetWin:
		return "Bet won"
	case events.EventBetLose:
		return "Bet lost"
	case events.EventBetRefund:
		return "Bet refunded"
	case events.EventBetFilters:
		return "Bet skipped"
	case events.EventBetGeneral:
		return "Bet"
	case events.EventBetFailed:
		return "Bet failed"
	case events.EventBetStart:
		return "Bet starting"
	case events.EventMomentClaim:
		return "Moment claimed"
	case events.EventJoinRaid:
		return "Raid joined"
	case events.EventDropClaim:
		return "Drop claimed"
	case events.EventDropStatus:
		return "Drop progress"
	case events.EventChatMention:
		return "Chat mention"
	default:
		if event == "" {
			return "Miner"
		}
		return string(event)
	}
}

func eventColor(event events.Event) int {
	switch event {
	case events.EventStreamerOnline, events.EventGainForWatch, events.EventGainForWatchStreak,
		events.EventBetWin, events.EventGainForClaim, events.EventBonusClaim:
		return 0x0A7A2F
	case events.EventStreamerOffline, events.EventBetLose, events.EventBetFailed:
		return 0xC23030
	case events.EventBetRefund:
		return 0xEDA100
	case events.EventDropClaim, events.EventDropStatus, events.EventMomentClaim:
		return 0xE87BA4
	case events.EventJoinRaid, events.EventGainForRaid:
		return 0x2A78D6
	case events.EventChatMention, events.EventBetStart, events.EventBetGeneral:
		return 0x6250D6
	default:
		return 0x5C5C66
	}
}

// eventGlyphPrefix is the polarity mark shared by Telegram and Discord ("🟢 ").
// Empty when the event has no glyph. Console `emojis` does not apply here.
func eventGlyphPrefix(event events.Event) string {
	emoji, _, ok := events.EventGlyph(event)
	if !ok || emoji == "" {
		return ""
	}
	return emoji + " "
}
