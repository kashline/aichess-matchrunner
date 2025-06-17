package util

import (
	"testing"
)

func TestDetermineScore(t *testing.T) {
	tests := []struct {
		name    string
		history []History
		want    int
	}{
		{
			name:    "empty history",
			history: []History{},
			want:    0,
		},
		{
			name: "single move white",
			history: []History{
				{Score: 10},
			},
			want: 10,
		},
		{
			name: "two moves white then black",
			history: []History{
				{Score: 10}, // white adds 10
				{Score: 5},  // black subtracts 5
			},
			want: 5, // 10 - 5 = 5
		},
		{
			name: "multiple moves",
			history: []History{
				{Score: 3}, // +3
				{Score: 2}, // -2
				{Score: 4}, // +4
				{Score: 1}, // -1
			},
			want: 4, // 3-2+4-1 = 4
		},
		{
			name: "negative scores",
			history: []History{
				{Score: -1}, // -1
				{Score: -2}, // +2
			},
			want: 1, // -1 - (-2) = -1 + 2 = 1
		},
	}

	for _, tt := range tests {
		got := DetermineScore(tt.history)
		if got != tt.want {
			t.Errorf("%s: DetermineScore() = %d, want %d", tt.name, got, tt.want)
		}
	}
}
