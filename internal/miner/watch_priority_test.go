package miner

import (
	"testing"
)

func TestParseWatchPriorities(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []watchPriority
	}{
		{
			name: "defaults when empty",
			in:   nil,
			want: defaultWatchPriorities(),
		},
		{
			name: "ignores unknowns and deduplicates",
			in:   []string{"drops", "ORDER", "drops", "points_desc", "ignored"},
			want: []watchPriority{
				watchPriorityDrops,
				watchPriorityOrder,
				watchPriorityPointsDescending,
			},
		},
		{
			name: "falls back to defaults when nothing recognized",
			in:   []string{"foo", "bar"},
			want: defaultWatchPriorities(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWatchPriorities(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d got %v want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
