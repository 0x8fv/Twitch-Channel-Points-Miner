package twitchchannelpointsminer

import (
	"sync/atomic"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/notify"
)

// eventSender is what the logger needs from a notifier.
type eventSender interface {
	Send(message string, event constants.Event)
}

var telegramSender atomic.Pointer[eventSender]

// SetTelegram installs the process-wide Telegram notifier; incomplete settings disable it.
func SetTelegram(settings notify.TelegramSettings) {
	if notifier := notify.NewTelegramNotifier(settings); notifier != nil {
		setTelegramSender(notifier)
		return
	}
	telegramSender.Store(nil)
}

func setTelegramSender(sender eventSender) {
	telegramSender.Store(&sender)
}

// telegramEmitEvent forwards a logged event (ANSI colors stripped) to Telegram.
func telegramEmitEvent(event constants.Event, message string) {
	if sender := telegramSender.Load(); sender != nil {
		(*sender).Send(ansiRegexp.ReplaceAllString(message, ""), event)
	}
}
