package notify

import (
	"strings"
	"unicode"
)

func rewriteGain(head, game, streak string) (string, bool) {
	arrow := strings.Index(head, "→")
	if arrow < 0 {
		arrow = strings.Index(head, "->")
	}
	if arrow <= 0 {
		return "", false
	}
	sep := "→"
	if head[arrow] == '-' {
		sep = "->"
	}
	amount := strings.TrimSpace(head[:arrow])
	if amount == "" || !isSignedAmount(amount) {
		return "", false
	}
	rest := strings.TrimSpace(head[arrow+len(sep):])
	reason := ""
	if i := indexFold(rest, " - reason:"); i >= 0 {
		reason = strings.TrimSpace(rest[i+len(" - reason:"):])
		rest = strings.TrimSpace(rest[:i])
	}
	name, points := splitPoints(rest)
	if name == "" {
		return "", false
	}
	sentence := gainSentence(name, amount, points, reason, game)
	if streak != "" {
		sentence += " The watch streak is " + streak + "."
	}
	return sentence, true
}

func gainSentence(name, amount, points, reason, game string) string {
	n, lost := unsignedAmount(amount)
	var b strings.Builder
	b.WriteString(name)
	if lost {
		b.WriteString(" lost ")
	} else {
		b.WriteString(" gained ")
	}
	b.WriteString(n)
	b.WriteString(" points")
	b.WriteString(reasonClause(reason, game))
	if points != "" {
		b.WriteString(" and now has ")
		b.WriteString(points)
	}
	b.WriteByte('.')
	return b.String()
}

func reasonClause(reason, game string) string {
	kind := ""
	if fields := strings.Fields(reason); len(fields) > 0 {
		kind = strings.ToUpper(fields[0])
	}
	switch kind {
	case "WATCH":
		if game != "" {
			return " for watching " + game
		}
		return " for watching"
	case "WATCH_STREAK":
		if game != "" {
			return " for a watch streak while playing " + game
		}
		return " for a watch streak"
	case "CLAIM":
		return " from a bonus claim"
	case "RAID":
		if game != "" {
			return " for a raid while playing " + game
		}
		return " for a raid"
	default:
		label := strings.ToLower(strings.ReplaceAll(kind, "_", " "))
		switch {
		case label != "" && game != "":
			return " (" + label + ") while playing " + game
		case label != "":
			return " (" + label + ")"
		case game != "":
			return " while playing " + game
		default:
			return ""
		}
	}
}

func unsignedAmount(amount string) (string, bool) {
	amount = strings.TrimSpace(amount)
	lost := strings.HasPrefix(amount, "-") || strings.HasPrefix(amount, "−")
	return strings.TrimLeft(amount, "+-−"), lost
}

func isSignedAmount(s string) bool {
	r := []rune(s)
	if len(r) < 2 {
		return false
	}
	if r[0] != '+' && r[0] != '-' && r[0] != '−' {
		return false
	}
	return unicode.IsDigit(r[1])
}
