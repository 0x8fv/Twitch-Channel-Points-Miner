// Package jsonvalue reads loosely typed values out of decoded JSON.
package jsonvalue

import (
	"encoding/json"
	"strings"
)

// Float reads a number decoded from JSON, whatever numeric type it arrived as.
func Float(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

// String returns v when it is a string, and "" otherwise.
func String(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Bool returns v when it is a bool, and false otherwise.
func Bool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// At follows a dot-separated path through nested objects and returns nil when any step is missing.
func At(data interface{}, path string) interface{} {
	if data == nil {
		return nil
	}
	current := data
	parts := strings.Split(path, ".")
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[p]
		if current == nil {
			return nil
		}
	}
	return current
}

// FirstString returns the first non-empty string found under one of keys.
func FirstString(data map[string]interface{}, keys ...string) string {
	if data == nil {
		return ""
	}
	for _, key := range keys {
		if val, ok := data[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// FirstInt returns the first non-zero number found under one of keys.
func FirstInt(data map[string]interface{}, keys ...string) int {
	if data == nil {
		return 0
	}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if intVal := int(Float(val)); intVal != 0 {
				return intVal
			}
		}
	}
	return 0
}
