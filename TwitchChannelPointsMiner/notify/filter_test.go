package notify

import (
	"testing"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

func TestWantsEvent(t *testing.T) {
	if wantsEvent(nil, constants.EventGainForWatch) || !wantsEvent(nil, constants.EventStreamerOnline) {
		t.Fatal("empty selection should skip quiet events only")
	}
	all := map[constants.Event]struct{}{"*": {}}
	if !wantsEvent(all, constants.EventDropStatus) {
		t.Fatal("* should include quiet events")
	}
	named := map[constants.Event]struct{}{constants.EventGainForWatch: {}}
	if !wantsEvent(named, constants.EventGainForWatch) || wantsEvent(named, constants.EventStreamerOnline) {
		t.Fatal("an explicit list should be exact")
	}
	if wantsEvent(nil, "") {
		t.Fatal("blank event is never selected")
	}
}
