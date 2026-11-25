package notify

import (
	"strings"
)

func rewriteDropClaim(head string) (string, bool) {
	const prefix = "claim "
	if !strings.HasPrefix(strings.ToLower(head), prefix) {
		return "", false
	}
	rest := strings.TrimSpace(head[len(prefix):])
	// "Reward (Campaign) 60/60 (100%)"
	open := strings.Index(rest, " (")
	if open <= 0 {
		return "", false
	}
	closeRel := strings.Index(rest[open:], ") ")
	if closeRel < 0 {
		return "", false
	}
	reward := strings.TrimSpace(rest[:open])
	campaign := strings.TrimSpace(rest[open+2 : open+closeRel])
	progress := strings.TrimSpace(rest[open+closeRel+2:])
	if reward == "" || campaign == "" || progress == "" {
		return "", false
	}
	return "Claimed " + reward + " from the " + campaign + " campaign, now at " + progress + ".", true
}

func dropProgressSentence(pct, desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "The drop is " + pct + " complete."
	}
	if i := strings.LastIndex(desc, " / "); i > 0 {
		tail := strings.TrimSpace(desc[i+3:])
		head := strings.TrimSpace(desc[:i])
		if strings.Contains(tail, " of ") && strings.Contains(tail, "m") {
			watched := tail
			if a, b, ok := strings.Cut(tail, " of "); ok {
				watched = strings.TrimSpace(a) + " watched out of " + strings.TrimSpace(b)
			}
			return "The drop " + head + " is " + pct + " complete, with " + watched + "."
		}
	}
	return "The drop " + desc + " is " + pct + " complete."
}
