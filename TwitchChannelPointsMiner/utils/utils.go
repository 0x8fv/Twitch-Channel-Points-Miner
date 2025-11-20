package utils

import (
	"encoding/json"
	"os"
	"runtime"
)

func GetUserAgent(browser string) string {
	platform := "Linux"
	switch runtime.GOOS {
	case "windows":
		platform = "Windows"
	case "android":
		platform = "Android"
	}

	if agent, ok := UserAgents[platform][browser]; ok {
		return agent
	}
	for _, agent := range UserAgents[platform] {
		return agent
	}
	return ""
}

func SaveJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
