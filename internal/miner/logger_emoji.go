package miner

import (
	"TwitchChannelPointsMiner/internal/events"
)

var emojiMap = map[string]string{
	":alarm_clock:":              "⏰",
	":bar_chart:":                "📊",
	":chart_with_upwards_trend:": "📈",
	":four_leaf_clover:":         "🍀",
	":rocket:":                   "🚀",
	":moneybag:":                 "💰",
	":green_circle:":             "🟢",
	":white_check_mark:":         "✅",
	":package:":                  "📦",
	":hourglass:":                "⌛",
	":hourglass_flowing_sand:":   "⏳",
	":speech_balloon:":           "💬",
	":partying_face:":            "🥳",
	":sleeping:":                 "😴",
	":stop_sign:":                "🛑",
	":page_facing_up:":           "📄",
	":gift:":                     "🎁",
	":clipboard:":                "📋",
	":performing_arts:":          "🎭",
	":cry:":                      "😢",
	":disappointed_relieved:":    "😥",
	":video_game:":               "🎮",
	":ambulance:":                "🚑",
}

func emojize(code string) string {
	if val, ok := emojiMap[code]; ok {
		return val
	}
	return code
}

// emojiFor picks the canonical glyph of a known event. Caller shortcodes
// (":rocket:", ":partying_face:") are only a fallback for lines with no event.
func emojiFor(code string, event events.Event) string {
	if emoji, _, ok := events.EventGlyph(event); ok {
		return emoji
	}
	if code == "" {
		return ""
	}
	return emojize(code)
}
