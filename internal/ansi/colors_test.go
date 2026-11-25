package ansi

import (
	"testing"
)

func TestColorConstantsNonEmpty(t *testing.T) {
	if ColorGreen == "" || ColorRed == "" || ColorCyan == "" || ColorPurple == "" || ColorYellow == "" || ColorReset == "" {
		t.Fatalf("color constants should not be empty")
	}
}
