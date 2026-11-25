package dashboard

import (
	"testing"
)

func TestDownsampleKeepsEndsAndExtremes(t *testing.T) {
	points := make([]Point, 10000)
	for i := range points {
		points[i] = Point{T: int64(i), P: 1000 + i%7}
	}
	points[4321].P = 999999 // spike
	points[8765].P = -5     // dip

	out := Downsample(points, MaxSeriesPoints)
	if len(out) > MaxSeriesPoints {
		t.Fatalf("got %d points, want <= %d", len(out), MaxSeriesPoints)
	}
	if out[0] != points[0] || out[len(out)-1] != points[len(points)-1] {
		t.Fatalf("first/last point not preserved")
	}
	var spike, dip bool
	for i, p := range out {
		if i > 0 && p.T <= out[i-1].T {
			t.Fatalf("points out of order at %d", i)
		}
		spike = spike || p.P == 999999
		dip = dip || p.P == -5
	}
	if !spike || !dip {
		t.Fatalf("extremes lost: spike=%v dip=%v", spike, dip)
	}
}

func TestDownsampleSmallInputUnchanged(t *testing.T) {
	points := []Point{{T: 1, P: 1}, {T: 2, P: 2}, {T: 3, P: 3}}
	if out := Downsample(points, MaxSeriesPoints); len(out) != 3 {
		t.Fatalf("got %d points, want 3", len(out))
	}
	if out := Downsample(nil, MaxSeriesPoints); len(out) != 0 {
		t.Fatalf("nil input should stay empty")
	}
}
