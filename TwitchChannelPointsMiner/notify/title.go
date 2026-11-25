package notify

import (
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

func eventTitle(event constants.Event) string {
	switch event {
	case constants.EventStartup:
		return "Session started"
	case constants.EventShutdown:
		return "Session ended"
	case constants.EventStreamerOnline:
		return "Online"
	case constants.EventStreamerOffline:
		return "Offline"
	case constants.EventGainForRaid:
		return "Raid points"
	case constants.EventGainForClaim, constants.EventBonusClaim:
		return "Bonus claimed"
	case constants.EventGainForWatch:
		return "Watch"
	case constants.EventGainForWatchStreak:
		return "Watch streak"
	case constants.EventBetWin:
		return "Bet won"
	case constants.EventBetLose:
		return "Bet lost"
	case constants.EventBetRefund:
		return "Bet refunded"
	case constants.EventBetFilters:
		return "Bet skipped"
	case constants.EventBetGeneral:
		return "Bet"
	case constants.EventBetFailed:
		return "Bet failed"
	case constants.EventBetStart:
		return "Bet starting"
	case constants.EventMomentClaim:
		return "Moment claimed"
	case constants.EventJoinRaid:
		return "Raid joined"
	case constants.EventDropClaim:
		return "Drop claimed"
	case constants.EventDropStatus:
		return "Drop progress"
	case constants.EventChatMention:
		return "Chat mention"
	default:
		if event == "" {
			return "Miner"
		}
		return string(event)
	}
}

func eventColor(event constants.Event) int {
	switch event {
	case constants.EventStreamerOnline, constants.EventGainForWatch, constants.EventGainForWatchStreak,
		constants.EventBetWin, constants.EventGainForClaim, constants.EventBonusClaim:
		return 0x0A7A2F
	case constants.EventStreamerOffline, constants.EventBetLose, constants.EventBetFailed:
		return 0xC23030
	case constants.EventBetRefund:
		return 0xEDA100
	case constants.EventDropClaim, constants.EventDropStatus, constants.EventMomentClaim:
		return 0xE87BA4
	case constants.EventJoinRaid, constants.EventGainForRaid:
		return 0x2A78D6
	case constants.EventChatMention, constants.EventBetStart, constants.EventBetGeneral:
		return 0x6250D6
	default:
		return 0x5C5C66
	}
}

// eventGlyphPrefix is the polarity mark shared by Telegram and Discord ("🟢 ").
// Empty when the event has no glyph. Console `emojis` does not apply here.
func eventGlyphPrefix(event constants.Event) string {
	emoji, _, ok := constants.EventGlyph(event)
	if !ok || emoji == "" {
		return ""
	}
	return emoji + " "
}
