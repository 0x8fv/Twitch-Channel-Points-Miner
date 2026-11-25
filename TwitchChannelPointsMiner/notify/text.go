package notify

import (
	"regexp"
	"strings"
	"unicode"
)

// ansiCodes matches the colour escapes the console logger adds to event lines.
var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripLeadingEmoji drops the glyph the console logger puts in front of a line.
// Every glyph it uses is a "symbol, other" rune, optionally followed by a
// variation selector or joined to another one.
func stripLeadingEmoji(s string) string {
	rest := strings.TrimLeftFunc(s, func(r rune) bool {
		return unicode.Is(unicode.So, r) || r == '️' || r == '‍'
	})
	if len(rest) == len(s) {
		return s
	}
	return strings.TrimSpace(rest)
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
