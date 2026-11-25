package twitchchannelpointsminer

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
)

// notifyBodyLimit caps a notification sentence. The console line stays complete.
const notifyBodyLimit = 500

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

func eventTitle(event constants.Event) string {
	switch event {
	case constants.EventStartup:
		return "Session started"
	case constants.EventShutdown:
		return "Session ended"
	case constants.EventStreamerOnline:
		return "Online"
	case constants.EventStreamerOffline:
		return "Offline"
	case constants.EventGainForRaid:
		return "Raid points"
	case constants.EventGainForClaim, constants.EventBonusClaim:
		return "Bonus claimed"
	case constants.EventGainForWatch:
		return "Watch"
	case constants.EventGainForWatchStreak:
		return "Watch streak"
	case constants.EventBetWin:
		return "Bet won"
	case constants.EventBetLose:
		return "Bet lost"
	case constants.EventBetRefund:
		return "Bet refunded"
	case constants.EventBetFilters:
		return "Bet skipped"
	case constants.EventBetGeneral:
		return "Bet"
	case constants.EventBetFailed:
		return "Bet failed"
	case constants.EventBetStart:
		return "Bet starting"
	case constants.EventMomentClaim:
		return "Moment claimed"
	case constants.EventJoinRaid:
		return "Raid joined"
	case constants.EventDropClaim:
		return "Drop claimed"
	case constants.EventDropStatus:
		return "Drop progress"
	case constants.EventChatMention:
		return "Chat mention"
	default:
		if event == "" {
			return "Miner"
		}
		return string(event)
	}
}

func eventColor(event constants.Event) int {
	switch event {
	case constants.EventStreamerOnline, constants.EventGainForWatch, constants.EventGainForWatchStreak,
		constants.EventBetWin, constants.EventGainForClaim, constants.EventBonusClaim:
		return 0x0A7A2F
	case constants.EventStreamerOffline, constants.EventBetLose, constants.EventBetFailed:
		return 0xC23030
	case constants.EventBetRefund:
		return 0xEDA100
	case constants.EventDropClaim, constants.EventDropStatus, constants.EventMomentClaim:
		return 0xE87BA4
	case constants.EventJoinRaid, constants.EventGainForRaid:
		return 0x2A78D6
	case constants.EventChatMention, constants.EventBetStart, constants.EventBetGeneral:
		return 0x6250D6
	default:
		return 0x5C5C66
	}
}

var (
	reDropBar    = regexp.MustCompile(`(?i)^DROP_PROGRESS\s+\[[#\-]*\]\s*(\d+%)\s*(?:\|\s*(.*))?$`)
	reChoiceLead = regexp.MustCompile(`^(?:[A-Z]|#\d+):\s*(.+)$`)
	reColorTail  = regexp.MustCompile(`\s+\([A-Z]+\)$`)
)

// eventSentence turns a console event line into one or two plain sentences.
// The streamer name stays in the text so analytics can still attribute the
// event. Safe to call more than once. Console output is unchanged.
func eventSentence(message string) string {
	s := strings.TrimSpace(ansiRegexp.ReplaceAllString(message, ""))
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
		return limitRunes(body, notifyBodyLimit)
	}
	if body, ok := rewriteOnline(head, game, streak, "offline"); ok {
		return limitRunes(body, notifyBodyLimit)
	}
	if body, ok := rewriteGain(head, game, streak); ok {
		return limitRunes(body, notifyBodyLimit)
	}
	if body, ok := rewritePrediction(head); ok {
		return limitRunes(body, notifyBodyLimit)
	}
	if body, ok := rewritePlace(head); ok {
		return limitRunes(body, notifyBodyLimit)
	}
	if strings.HasPrefix(lower, "place bet after ") {
		rest := strings.TrimSpace(head[len("place bet after "):])
		when, who, ok := splitLastFold(rest, " for ")
		if ok {
			return limitRunes("The bet for "+who+" will be placed in "+when+".", notifyBodyLimit)
		}
	}
	if who, reason, ok := splitOnceFold(head, "skip bet for ", ": "); ok {
		return limitRunes(skipSentence(who, reason), notifyBodyLimit)
	}
	if name, detail, ok := splitOnceFold(head, "prediction ", ": "); ok {
		detail = strings.TrimRight(strings.TrimSpace(detail), ".")
		if detail == "" {
			return limitRunes("The prediction for "+name+" failed.", notifyBodyLimit)
		}
		return limitRunes("The prediction for "+name+" failed: "+detail+".", notifyBodyLimit)
	}
	if from, to, ok := splitOnceFold(head, "joining raid from ", " to "); ok {
		return limitRunes("Joining the raid from "+from+" to "+to+".", notifyBodyLimit)
	}
	if name, ok := cutSuffixFold(head, " claimed moment"); ok {
		name = strings.TrimSpace(name)
		if name == "" {
			return "A moment was claimed."
		}
		return limitRunes(name+" claimed a moment.", notifyBodyLimit)
	}
	if body, ok := rewriteDropClaim(head); ok {
		return limitRunes(body, notifyBodyLimit)
	}
	if m := reDropBar.FindStringSubmatch(head); m != nil {
		desc := strings.TrimSpace(m[2])
		if desc == "" {
			desc = strings.Join(other, " | ")
		}
		if i := strings.Index(strings.ToLower(desc), " / remaining "); i >= 0 {
			desc = desc[:i]
		}
		return limitRunes(dropProgressSentence(m[1], desc), notifyBodyLimit)
	}
	if nick, channel, wrote, ok := rewriteMention(head); ok {
		wrote = strings.TrimSpace(wrote)
		return limitRunes(nick+" mentioned you in #"+channel+`: "`+wrote+`".`, notifyBodyLimit)
	}

	return limitRunes(fallbackSentence(head, game, streak, other), notifyBodyLimit)
}

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

func stripLeadingEmoji(s string) string {
	for _, emoji := range emojiMap {
		if strings.HasPrefix(s, emoji) {
			return strings.TrimSpace(strings.TrimPrefix(s, emoji))
		}
	}
	return s
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func indexFold(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}

func splitLastFold(s, sep string) (string, string, bool) {
	i := strings.LastIndex(strings.ToLower(s), strings.ToLower(sep))
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(sep):]), true
}

func splitOnceFold(s, prefix, sep string) (string, string, bool) {
	if !strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
		return "", "", false
	}
	rest := strings.TrimSpace(s[len(prefix):])
	i := indexFold(rest, sep)
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(rest[:i]), strings.TrimSpace(rest[i+len(sep):]), true
}

func cutSuffixFold(s, suffix string) (string, bool) {
	if !strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix)) {
		return "", false
	}
	return s[:len(s)-len(suffix)], true
}

func replaceFold(s, old, new string) string {
	lower := strings.ToLower(s)
	oldLower := strings.ToLower(old)
	var b strings.Builder
	for {
		i := strings.Index(lower, oldLower)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(new)
		s = s[i+len(old):]
		lower = lower[i+len(oldLower):]
	}
}

func limitRunes(s string, n int) string {
	r := []rune(s)
	if n < 1 || len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// formatTelegram renders a console event as a title plus a sentence (parse_mode HTML).
func formatTelegram(event constants.Event, message string) string {
	title := "<b>" + htmlEscape(eventTitle(event)) + "</b>"
	body := eventSentence(message)
	if body == "" || strings.EqualFold(body, eventTitle(event)) {
		return limitRunes(title, telegramTextLimit)
	}
	escaped := htmlEscape(body)
	budget := telegramTextLimit - len([]rune(title)) - 1
	if budget < 1 {
		return limitRunes(title, telegramTextLimit)
	}
	return title + "\n" + limitEscaped(escaped, budget)
}

// limitEscaped truncates an HTML-escaped string without splitting an entity.
func limitEscaped(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	cut := string(r[:n-1])
	if i := strings.LastIndex(cut, "&"); i >= 0 && !strings.Contains(cut[i:], ";") {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}
