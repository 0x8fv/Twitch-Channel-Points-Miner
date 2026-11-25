package notify

import (
	"strings"
)

func rewritePrediction(head string) (string, bool) {
	const prefix = "eventprediction:"
	if !strings.HasPrefix(strings.ToLower(head), prefix) {
		return "", false
	}
	rest := strings.TrimSpace(head[len(prefix):])
	var name, title, choice, result string
	for _, part := range strings.Split(rest, " - ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "decision:"):
			choice = shortOutcome(strings.TrimSpace(part[len("decision:"):]))
		case strings.HasPrefix(lower, "result:"):
			result = strings.TrimSpace(part[len("result:"):])
			result = replaceFold(result, ", gained: ", " ")
			result = replaceFold(result, ", lost: ", " ")
			result = replaceFold(result, ", refunded: ", " ")
			result = collapseSpace(result)
		case name == "":
			name = part
		case title == "":
			title = part
		default:
			title = strings.TrimSpace(title + " " + part)
		}
	}
	if name == "" {
		return "", false
	}
	return predictionSentence(name, title, choice, result), true
}

func predictionSentence(name, title, choice, result string) string {
	kind, pts := splitResult(result)
	var b strings.Builder
	b.WriteString(name)
	switch kind {
	case "WIN":
		if pts != "" {
			b.WriteString(" won ")
			b.WriteString(pts)
			b.WriteString(" points")
		} else {
			b.WriteString(" won the bet")
		}
	case "LOSE":
		if pts != "" {
			b.WriteString(" lost ")
			b.WriteString(pts)
			b.WriteString(" points")
		} else {
			b.WriteString(" lost the bet")
		}
	case "REFUND":
		if pts != "" {
			b.WriteString(" was refunded ")
			b.WriteString(pts)
			b.WriteString(" points")
		} else {
			b.WriteString(" was refunded")
		}
	default:
		b.WriteString(" finished a bet")
		if result != "" {
			b.WriteString(" (")
			b.WriteString(result)
			b.WriteString(")")
		}
	}
	if title != "" {
		b.WriteString(" on \"")
		b.WriteString(title)
		b.WriteByte('"')
	}
	if choice != "" {
		b.WriteString(" after choosing ")
		b.WriteString(choice)
	}
	b.WriteByte('.')
	return b.String()
}

func splitResult(result string) (kind, pts string) {
	fields := strings.Fields(result)
	if len(fields) == 0 {
		return "", ""
	}
	kind = strings.ToUpper(fields[0])
	switch kind {
	case "WIN", "LOSE", "REFUND":
	default:
		return "", result
	}
	if len(fields) >= 2 {
		pts = strings.TrimLeft(fields[len(fields)-1], "+-−")
	}
	return kind, pts
}

func rewritePlace(head string) (string, bool) {
	lower := strings.ToLower(head)
	amount := ""
	var rest string
	switch {
	case strings.HasPrefix(lower, "place bet on:"):
		rest = strings.TrimSpace(head[len("place bet on:"):])
	case strings.HasPrefix(lower, "place ") && strings.Contains(lower, " points on:"):
		i := strings.Index(lower, " points on:")
		amount = strings.TrimSpace(head[len("place "):i])
		rest = strings.TrimSpace(head[i+len(" points on:"):])
	default:
		return "", false
	}
	outcome, who, ok := splitLastFold(rest, " for ")
	if !ok {
		return "", false
	}
	outcome = shortOutcome(outcome)
	switch {
	case amount != "" && outcome != "":
		return who + " placed " + amount + " points on " + outcome + ".", true
	case outcome != "":
		return who + " placed a bet on " + outcome + ".", true
	case amount != "":
		return who + " placed " + amount + " points.", true
	default:
		return who + " placed a bet.", true
	}
}

func skipSentence(who, reason string) string {
	reason = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(reason), "_", " "))
	switch {
	case reason == "":
		return "The bet for " + who + " was skipped."
	case strings.Contains(reason, "no outcome"):
		return "The bet for " + who + " was skipped because no outcome was selected."
	case strings.Contains(reason, "twitch"):
		return "The bet for " + who + " was skipped because the stake is below Twitch's minimum."
	case strings.Contains(reason, "minimum"):
		return "The bet for " + who + " was skipped because the balance is below the minimum."
	case strings.Contains(reason, "filter"):
		return "The bet for " + who + " was skipped because the bet filter did not match."
	case strings.Contains(reason, "status"):
		return "The bet for " + who + " was skipped because the prediction is no longer active."
	default:
		if !strings.HasSuffix(reason, ".") {
			reason += "."
		}
		return "The bet for " + who + " was skipped because " + reason
	}
}
