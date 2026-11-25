package prediction

import (
	"fmt"
	"strings"
)

func FormatNumber(value int) string {
	sign := ""
	v := value
	if v < 0 {
		sign = "-"
		v = -v
	}
	switch {
	case v >= 1_000_000:
		return sign + trimZeros(fmt.Sprintf("%.2fM", float64(v)/1_000_000))
	case v >= 1_000:
		return sign + trimZeros(fmt.Sprintf("%.2fk", float64(v)/1_000))
	default:
		return fmt.Sprintf("%s%d", sign, v)
	}
}

func FormatFloat(val float64) string {
	return trimZeros(fmt.Sprintf("%.2f", val))
}

func trimZeros(val string) string {
	val = strings.TrimRight(strings.TrimRight(val, "0"), ".")
	if val == "" || val == "-" {
		return "0"
	}
	return val
}
