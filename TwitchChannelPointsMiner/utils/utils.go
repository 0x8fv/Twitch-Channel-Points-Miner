package utils

import (
	"encoding/json"
	"os"
)

func GetUserAgent(_ string) string {
	return UserAgents["Android"]["TV"]
}

func SaveJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
