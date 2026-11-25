package notify

import (
	"strings"
)

func rewriteOnline(head, game, streak, verb string) (string, bool) {
	lower := strings.ToLower(head)
	marker := " is " + verb
	i := strings.LastIndex(lower, marker)
	if i <= 0 {
		return "", false
	}
	tail := lower[i+len(marker):]
	if tail != "" && tail != "!" {
		return "", false
	}
	name, points := splitPoints(strings.TrimSpace(head[:i]))
	if name == "" {
		name = "The channel"
	}
	return presenceSentence(name, points, game, streak, verb), true
}

func presenceSentence(name, points, game, streak, verb string) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(" is ")
	b.WriteString(verb)
	if points != "" {
		b.WriteString(" with ")
		b.WriteString(points)
	}
	if game != "" {
		b.WriteString(", playing ")
		b.WriteString(game)
	}
	b.WriteByte('.')
	if streak != "" {
		b.WriteString(" The watch streak is ")
		b.WriteString(streak)
		b.WriteByte('.')
	}
	return b.String()
}

func splitPoints(subject string) (name, points string) {
	open := strings.LastIndex(subject, "(")
	if open <= 0 || !strings.HasSuffix(subject, ")") {
		return subject, ""
	}
	inner := subject[open+1 : len(subject)-1]
	if !strings.Contains(strings.ToLower(inner), "point") {
		return subject, ""
	}
	return strings.TrimSpace(subject[:open]), strings.TrimSpace(inner)
}
