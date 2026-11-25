package notify

import (
	"strings"
)

func rewriteMention(head string) (nick, channel, wrote string, ok bool) {
	const wroteMark = " wrote: "
	i := indexFold(head, wroteMark)
	if i < 0 {
		return "", "", "", false
	}
	left := head[:i]
	wrote = strings.TrimSpace(head[i+len(wroteMark):])
	at := indexFold(left, " at #")
	if at <= 0 {
		return "", "", "", false
	}
	nick = strings.TrimSpace(left[:at])
	channel = strings.TrimSpace(left[at+len(" at #"):])
	if nick == "" || channel == "" {
		return "", "", "", false
	}
	return nick, channel, wrote, true
}
