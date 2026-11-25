package notify

import (
	"testing"

	"TwitchChannelPointsMiner/internal/events"
)

func TestWantsEvent(t *testing.T) {
	if wantsEvent(nil, events.EventGainForWatch) || !wantsEvent(nil, events.EventStreamerOnline) {
		t.Fatal("empty selection should skip quiet events only")
	}
	all := map[events.Event]struct{}{"*": {}}
	if !wantsEvent(all, events.EventDropStatus) {
		t.Fatal("* should include quiet events")
	}
	named := map[events.Event]struct{}{events.EventGainForWatch: {}}
	if !wantsEvent(named, events.EventGainForWatch) || wantsEvent(named, events.EventStreamerOnline) {
		t.Fatal("an explicit list should be exact")
	}
	if wantsEvent(nil, "") {
		t.Fatal("blank event is never selected")
	}
}
