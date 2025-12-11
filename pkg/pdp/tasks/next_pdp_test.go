package tasks

import (
	"testing"
)

func TestAdjustNextProveAt(t *testing.T) {
	tests := []struct {
		name            string
		nextProveAt     int64
		currentHeight   int64
		finality        int64
		provingPeriod   int64
		challengeWindow int64
		expected        int64
	}{
		{
			name:            "stale epoch corrected into same window",
			nextProveAt:     900,
			currentHeight:   1000,
			finality:        5,
			provingPeriod:   60,
			challengeWindow: 20,
			expected:        1020,
		},
		{
			name:            "skip periods to meet requirement",
			nextProveAt:     3272185,
			currentHeight:   3272163,
			finality:        120,
			provingPeriod:   240,
			challengeWindow: 20,
			expected:        3272425,
		},
		{
			name:            "handles tiny window clamp to end",
			nextProveAt:     5000,
			currentHeight:   4980,
			finality:        10,
			provingPeriod:   30,
			challengeWindow: 5,
			expected:        5000,
		},
		{
			name:            "clamps inside current window to meet finality",
			nextProveAt:     1000,
			currentHeight:   1010,
			finality:        15,
			provingPeriod:   100,
			challengeWindow: 50,
			expected:        1025,
		},
		{
			name:            "metadata missing falls back to min required",
			nextProveAt:     100,
			currentHeight:   200,
			finality:        5,
			provingPeriod:   0,
			challengeWindow: 0,
			expected:        205,
		},
		{
			name:            "already future epoch left unchanged",
			nextProveAt:     4000,
			currentHeight:   3900,
			finality:        20,
			provingPeriod:   200,
			challengeWindow: 50,
			expected:        4000,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			minRequired := tt.currentHeight + tt.finality
			result := adjustNextProveAt(
				tt.nextProveAt,
				minRequired,
				tt.provingPeriod,
				tt.challengeWindow,
			)

			got := result
			if got != tt.expected {
				t.Fatalf("adjustNextProveAt() = %d, expected %d", got, tt.expected)
			}

			if got < minRequired {
				t.Fatalf("result %d should be >= minRequired %d", got, minRequired)
			}

			if tt.provingPeriod > 0 {
				windowStart := tt.nextProveAt
				windowEnd := windowStart + tt.challengeWindow
				for windowEnd < minRequired {
					windowStart += tt.provingPeriod
					windowEnd += tt.provingPeriod
				}
				if got < windowStart || got > windowEnd {
					t.Fatalf("result %d not within window [%d, %d]", got, windowStart, windowEnd)
				}
			}
		})
	}
}
