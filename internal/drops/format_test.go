package drops

import (
	"testing"
)

func TestFormatDropProgressBar(t *testing.T) {
	if got := formatDropProgressBar(63, 180); got != "[#######-------------]" {
		t.Fatalf("progress bar got %q", got)
	}
	if got := formatDropProgressBar(0, 0); got != "[--------------------]" {
		t.Fatalf("empty progress bar got %q", got)
	}
	if got := formatDropProgressBar(250, 180); got != "[####################]" {
		t.Fatalf("complete progress bar got %q", got)
	}
}

func TestFormatDropProgress(t *testing.T) {
	if got := formatDropProgress(3, 10); got != "3/10" {
		t.Fatalf("drop progress got %s", got)
	}
	if got := formatDropProgress(5, 0); got != "5" {
		t.Fatalf("drop progress with zero required got %s", got)
	}

	if got := progressPercent(5, 10); got != 50 {
		t.Fatalf("percent got %d", got)
	}
	if got := progressPercent(1, 0); got != 100 {
		t.Fatalf("zero required but progress present got %d", got)
	}
	if got := progressPercent(0, 0); got != 0 {
		t.Fatalf("zero current and required got %d", got)
	}
}
