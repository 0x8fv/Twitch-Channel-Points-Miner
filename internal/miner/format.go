package miner

import (
	"fmt"
	"strings"
	"time"
)

func formatChannelPoints(points int) string {
	value := points
	if value < 0 {
		value = -value
	}
	switch {
	case value >= 1_000_000:
		return formatPointsWithSuffix(value, 1_000_000, "M")
	case value >= 1_000:
		return formatPointsWithSuffix(value, 1_000, "k")
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatPointsWithSuffix(points int, divisor float64, suffix string) string {
	short := float64(points) / divisor
	formatted := fmt.Sprintf("%.2f", short)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	return formatted + suffix
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	d = d.Round(time.Second)
	day := 24 * time.Hour
	days := d / day
	d -= days * day
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%02dh", hours))
	}
	if minutes > 0 || len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%02dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%02ds", seconds))
	return strings.Join(parts, " ")
}
