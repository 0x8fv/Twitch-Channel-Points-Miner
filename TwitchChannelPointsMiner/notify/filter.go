package notify

import (
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

// quietEvents are high-frequency or low-signal. An empty events list sends
// every other event; name these (or "*") to opt in.
var quietEvents = map[constants.Event]struct{}{
	constants.EventGainForWatch: {},
	constants.EventDropStatus:   {},
	constants.EventBetFilters:   {},
}

// wantsEvent reports whether a notifier with the given selection should deliver
// event. An empty selection means every event except quietEvents. "*" or "ALL"
// means every event, including the quiet ones.
func wantsEvent(selected map[constants.Event]struct{}, event constants.Event) bool {
	if event == "" {
		return false
	}
	if len(selected) == 0 {
		_, quiet := quietEvents[event]
		return !quiet
	}
	if _, ok := selected["*"]; ok {
		return true
	}
	if _, ok := selected[constants.Event("ALL")]; ok {
		return true
	}
	_, ok := selected[event]
	return ok
}
