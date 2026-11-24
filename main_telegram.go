package main

import (
	"bytes"
	"encoding/json"
	"strings"

	miner "TwitchChannelPointsMiner/TwitchChannelPointsMiner"
)

// chatID accepts a JSON number (123456789, -1001234567890) or string ("@channel").
type chatID string

func (c *chatID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*c = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*c = chatID(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*c = chatID(n.String())
	return nil
}

type telegramConfig struct {
	ChatID              chatID   `json:"chat_id"`
	Token               string   `json:"token"`
	Events              []string `json:"events"`
	DisableNotification bool     `json:"disable_notification"`
}

func defaultTelegramConfig() map[string]interface{} {
	return map[string]interface{}{
		"chat_id":              "",
		"token":                "",
		"events":               []interface{}{},
		"disable_notification": false,
	}
}

func (c telegramConfig) settings() miner.TelegramSettings {
	return miner.TelegramSettings{
		ChatID:              string(c.ChatID),
		Token:               c.Token,
		Events:              c.Events,
		DisableNotification: c.DisableNotification,
	}
}
