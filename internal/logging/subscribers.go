package logging

import (
	"sync"

	"TwitchChannelPointsMiner/internal/events"
)

// A Subscriber receives every logged event as a plain sentence, without colour codes.
type Subscriber func(event events.Event, sentence string)

var subscribers sync.Map

// Subscribe installs fn under name, replacing an earlier subscriber of that name.
// A nil fn removes it. Subscribers must be safe for concurrent use and must not block for long.
func Subscribe(name string, fn Subscriber) {
	if fn == nil {
		subscribers.Delete(name)
		return
	}
	subscribers.Store(name, fn)
}

func publish(event events.Event, sentence string) {
	subscribers.Range(func(_, fn any) bool {
		fn.(Subscriber)(event, sentence)
		return true
	})
}
