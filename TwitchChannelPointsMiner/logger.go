package twitchchannelpointsminer

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type LoggerSettings struct {
	Save             bool `json:"save"`
	ConsoleLevel     int  `json:"console_level"`
	FileLevel        int  `json:"file_level"`
	Emoji            bool `json:"emoji"`
	Smart            bool `json:"smart"`
	ShowSeconds      bool `json:"show_seconds"`
	ConsoleUsername  bool `json:"console_username"`
	ShowClaimedBonus bool `json:"show_claimed_bonus_msg"`
	Less             bool `json:"less"`
	Debug            bool `json:"debug"`
	DebugDeep        bool `json:"debug_deep"`
	AnonymizeLogs    bool `json:"anonymize_logs"`
}

type Logger struct {
	base     *log.Logger
	settings LoggerSettings
}

type dualWriter struct {
	console io.Writer
	file    io.Writer
}

func (w dualWriter) Write(p []byte) (int, error) {
	if w.console != nil {
		if _, err := w.console.Write(p); err != nil {
			return 0, err
		}
	}
	if w.file != nil {
		clean := ansiRegexp.ReplaceAll(p, nil)
		if _, err := w.file.Write(clean); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func NewLogger(settings LoggerSettings, username string) *Logger {
	var output io.Writer = os.Stdout
	if settings.Save {
		logDir := "log"
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			name := strings.TrimSpace(username)
			if settings.AnonymizeLogs || name == "" {
				name = "miner"
			}
			name = sanitizeFilename(name)
			logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", name))
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				output = dualWriter{
					console: os.Stdout,
					file:    f,
				}
			}
		}
	}
	return &Logger{
		base:     log.New(output, "", 0),
		settings: settings,
	}
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}

func (l *Logger) log(level, emoji, format string, args ...interface{}) {
	if level == "DEBUG" && !l.settings.Debug {
		return
	}
	if level == "DEEP" && (!l.settings.Debug || !l.settings.DebugDeep) {
		return
	}
	message := fmt.Sprintf(format, args...)
	if emoji != "" && l.settings.Emoji {
		message = fmt.Sprintf("%s %s", emojize(emoji), message)
	}
	timestampFormat := "15:04 02/01/06"
	if l.settings.ShowSeconds {
		timestampFormat = "15:04:05 02/01/06"
	}
	timestamp := time.Now().Format(timestampFormat)
	l.base.Printf("[%s] %s: %s", level, timestamp, message)
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.log("INFO", "", format, args...)
}

func (l *Logger) Println(v ...interface{}) {
	l.log("INFO", "", "%s", fmt.Sprint(v...))
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log("ERROR", "", format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log("ERROR", "", format, args...)
	os.Exit(1)
}

func (l *Logger) EmojiPrintf(emoji, format string, args ...interface{}) {
	l.log("INFO", emoji, format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log("DEBUG", "", format, args...)
}

func (l *Logger) DebugEnabled() bool {
	return l.settings.Debug
}

func (l *Logger) DeepDebugf(format string, args ...interface{}) {
	l.log("DEEP", "", format, args...)
}

func (l *Logger) DeepDebugEnabled() bool {
	return l.settings.Debug && l.settings.DebugDeep
}

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
}

func emojize(code string) string {
	if val, ok := emojiMap[code]; ok {
		return val
	}
	return code
}
