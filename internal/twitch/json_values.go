package twitch

import (
	"encoding/json"
	"strings"
)

func mapStringValue(data map[string]interface{}, keys ...string) string {
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

func mapIntValue(data map[string]interface{}, keys ...string) int {
	if data == nil {
		return 0
	}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if intVal := int(fromFloat(val)); intVal != 0 {
				return intVal
			}
		}
	}
	return 0
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func fromFloat(v interface{}) float64 {
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

func navigate(data interface{}, path string) interface{} {
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
