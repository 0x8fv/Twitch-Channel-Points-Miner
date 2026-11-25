package dashboard

// MaxSeriesPoints is the most points a chart series is ever sent with.
const MaxSeriesPoints = 800

// Downsample reduces points to at most max entries. The first and last points
// are always kept; the rest is split into equal buckets, each contributing its
// minimum and maximum balance (in time order) so spikes and dips survive.
func Downsample(points []Point, max int) []Point {
	if max < 4 {
		max = 4
	}
	n := len(points)
	if n <= max {
		return points
	}
	buckets := (max - 2) / 2
	inner := points[1 : n-1]
	out := make([]Point, 0, max)
	out = append(out, points[0])
	for b := 0; b < buckets; b++ {
		lo := b * len(inner) / buckets
		hi := (b + 1) * len(inner) / buckets
		if lo >= hi {
			continue
		}
		minI, maxI := lo, lo
		for i := lo + 1; i < hi; i++ {
			if inner[i].P < inner[minI].P {
				minI = i
			}
			if inner[i].P > inner[maxI].P {
				maxI = i
			}
		}
		first, second := minI, maxI
		if first > second {
			first, second = second, first
		}
		out = append(out, inner[first])
		if second != first {
			out = append(out, inner[second])
		}
	}
	return append(out, points[n-1])
}
