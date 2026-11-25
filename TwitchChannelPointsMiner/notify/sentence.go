package notify

import (
	"regexp"
	"strconv"
	"strings"
)

// sentenceLimit caps a notification sentence. The console line stays complete.
const sentenceLimit = 500

var (
	reDropBar    = regexp.MustCompile(`(?i)^DROP_PROGRESS\s+\[[#\-]*\]\s*(\d+%)\s*(?:\|\s*(.*))?$`)
	reChoiceLead = regexp.MustCompile(`^(?:[A-Z]|#\d+):\s*(.+)$`)
	reColorTail  = regexp.MustCompile(`\s+\([A-Z]+\)$`)
)

// Sentence turns a console event line into one or two plain sentences.
// The streamer name stays in the text so analytics can still attribute the
// event. Safe to call more than once. Console output is unchanged.
func Sentence(message string) string {
	s := strings.TrimSpace(ansiCodes.ReplaceAllString(message, ""))
	s = stripLeadingEmoji(s)
	if s == "" {
		return ""
	}
	s = collapseSpace(s)

	head, game, streak, other := splitNotifyPipes(s)
	lower := strings.ToLower(head)

	switch {
	case strings.HasPrefix(lower, "start session:"):
		return "The mining session has started."
	case strings.HasPrefix(lower, "ending session:"):
		return "The mining session has ended."
	}

	if body, ok := rewriteOnline(head, game, streak, "online"); ok {
		return limitRunes(body, sentenceLimit)
	}
	if body, ok := rewriteOnline(head, game, streak, "offline"); ok {
		return limitRunes(body, sentenceLimit)
	}
	if body, ok := rewriteGain(head, game, streak); ok {
		return limitRunes(body, sentenceLimit)
	}
	if body, ok := rewritePrediction(head); ok {
		return limitRunes(body, sentenceLimit)
	}
	if body, ok := rewritePlace(head); ok {
		return limitRunes(body, sentenceLimit)
	}
	if strings.HasPrefix(lower, "place bet after ") {
		rest := strings.TrimSpace(head[len("place bet after "):])
		when, who, ok := splitLastFold(rest, " for ")
		if ok {
			return limitRunes("The bet for "+who+" will be placed in "+when+".", sentenceLimit)
		}
	}
	if who, reason, ok := splitOnceFold(head, "skip bet for ", ": "); ok {
		return limitRunes(skipSentence(who, reason), sentenceLimit)
	}
	if name, detail, ok := splitOnceFold(head, "prediction ", ": "); ok {
		detail = strings.TrimRight(strings.TrimSpace(detail), ".")
		if detail == "" {
			return limitRunes("The prediction for "+name+" failed.", sentenceLimit)
		}
		return limitRunes("The prediction for "+name+" failed: "+detail+".", sentenceLimit)
	}
	if from, to, ok := splitOnceFold(head, "joining raid from ", " to "); ok {
		return limitRunes("Joining the raid from "+from+" to "+to+".", sentenceLimit)
	}
	if name, ok := cutSuffixFold(head, " claimed moment"); ok {
		name = strings.TrimSpace(name)
		if name == "" {
			return "A moment was claimed."
		}
		return limitRunes(name+" claimed a moment.", sentenceLimit)
	}
	if body, ok := rewriteDropClaim(head); ok {
		return limitRunes(body, sentenceLimit)
	}
	if m := reDropBar.FindStringSubmatch(head); m != nil {
		desc := strings.TrimSpace(m[2])
		if desc == "" {
			desc = strings.Join(other, " | ")
		}
		if i := strings.Index(strings.ToLower(desc), " / remaining "); i >= 0 {
			desc = desc[:i]
		}
		return limitRunes(dropProgressSentence(m[1], desc), sentenceLimit)
	}
	if nick, channel, wrote, ok := rewriteMention(head); ok {
		wrote = strings.TrimSpace(wrote)
		return limitRunes(nick+" mentioned you in #"+channel+`: "`+wrote+`".`, sentenceLimit)
	}

	return limitRunes(fallbackSentence(head, game, streak, other), sentenceLimit)
}

// splitNotifyPipes keeps the main clause plus the game and a positive streak.
// The raw timestamps from LogExtraInfo are not part of a sentence worth sending.
func splitNotifyPipes(s string) (head, game, streak string, other []string) {
	parts := strings.Split(s, " | ")
	head = strings.TrimSpace(parts[0])
	for _, raw := range parts[1:] {
		p := strings.TrimSpace(raw)
		lower := strings.ToLower(p)
		switch {
		case strings.HasPrefix(lower, "playing:"), strings.HasPrefix(lower, "game:"):
			if v := strings.TrimSpace(p[strings.Index(p, ":")+1:]); v != "" && game == "" {
				game = v
			}
		case strings.HasPrefix(lower, "streak length "):
			n := strings.TrimSpace(p[len("streak length "):])
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				streak = n
			}
		case strings.HasPrefix(lower, "expires at "),
			strings.HasPrefix(lower, "started at "),
			strings.HasPrefix(lower, "streak achievement"):
			continue
		default:
			if p != "" {
				other = append(other, p)
			}
		}
	}
	return head, game, streak, other
}

func fallbackSentence(head, game, streak string, other []string) string {
	s := strings.TrimSpace(head)
	if s == "" {
		s = "Something changed"
	}
	if !strings.ContainsAny(s[len(s)-1:], ".!?") {
		s += "."
	}
	if game != "" && !strings.Contains(strings.ToLower(s), strings.ToLower(game)) {
		s += " The current game is " + game + "."
	}
	if streak != "" && !strings.Contains(strings.ToLower(s), "watch streak") {
		s += " The watch streak is " + streak + "."
	}
	for _, extra := range other {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		if !strings.ContainsAny(extra[len(extra)-1:], ".!?") {
			extra += "."
		}
		s += " " + extra
	}
	return s
}

func shortOutcome(s string) string {
	s = strings.TrimSpace(s)
	if i := indexFold(s, ", points:"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if m := reChoiceLead.FindStringSubmatch(s); m != nil {
		s = strings.TrimSpace(m[1])
	}
	s = reColorTail.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
