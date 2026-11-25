// Package jsonfile writes values to disk as indented JSON.
package jsonfile

import (
	"encoding/json"
	"os"
)

func SaveJSON(path string, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
