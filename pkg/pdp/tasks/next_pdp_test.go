package tasks

import (
	"math/big"
	"testing"
)

func TestAdjustNextProveAt(t *testing.T) {
	tests := []struct {
		name              string
		currentHeight     int64
		challengeFinality *big.Int
		challengeWindow   *big.Int
		expected          int64
		description       string
	}{
		{
			name:              "basic window boundary calculation",
			currentHeight:     1000,
			challengeFinality: big.NewInt(5),
			challengeWindow:   big.NewInt(8),
			expected:          1009, // minRequired=1005, next window=1008, result=1009 (1008+1)
			description:       "Should schedule 1 epoch after next window boundary",
		},
		{
			name:              "exact window boundary case",
			currentHeight:     2000,
			challengeFinality: big.NewInt(2),
			challengeWindow:   big.NewInt(30),
			expected:          2011, // minRequired=2002, next window=2010, result=2011 (2010+1)
			description:       "When minRequired doesn't fall on boundary, find next window",
		},
		{
			name:              "falls exactly on window boundary",
			currentHeight:     100,
			challengeFinality: big.NewInt(12), // 100+12=112, which is 7*16=112 exactly
			challengeWindow:   big.NewInt(16),
			expected:          129, // minRequired=112 (window boundary), next window=128, result=129 (128+1)
			description:       "When minRequired falls exactly on boundary, move to next window",
		},
		{
			name:              "mainnet params inside current window",
			currentHeight:     5568958,
			challengeFinality: big.NewInt(150), // mainnet challengeFinality
			challengeWindow:   big.NewInt(20),  // mainnet challenge window size
			expected:          5569121,         // minRequired=5569108, next window=5569120, result=5569121
			description:       "Large finality relative to window should advance to the next window boundary",
		},
		{
			name:              "mainnet params exact boundary",
			currentHeight:     1000010,         // (height + finality) lands exactly on a 20-epoch boundary
			challengeFinality: big.NewInt(150), // mainnet challengeFinality
			challengeWindow:   big.NewInt(20),  // mainnet challenge window size
			expected:          1000181,         // minRequired=1000160, boundary=1000160 => next window 1000180, result=1000181
			description:       "Exact boundary must bump to the next window and add 1 epoch",
		},
		{
			name:              "realistic small finality/window",
			currentHeight:     1000000,
			challengeFinality: big.NewInt(2),
			challengeWindow:   big.NewInt(30),
			expected:          1000021, // minRequired=1000002, next window=1000020, result=1000021
			description:       "Small finality with 30-epoch windows advances to the immediate next window",
		},
		{
			name:              "late in window still advances only one window",
			currentHeight:     2685164,
			challengeFinality: big.NewInt(2),
			challengeWindow:   big.NewInt(30),
			expected:          2685181, // minRequired=2685166, next window=2685180, result=2685181
			description:       "Late-window case advances to next boundary, not multiple windows ahead",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := adjustNextProveAt(tt.currentHeight, tt.challengeFinality, tt.challengeWindow)
			resultInt := result.Int64()

			// Check exact expected value
			if resultInt != tt.expected {
				t.Errorf("adjustNextProveAt() = %d, expected %d", resultInt, tt.expected)
			}

			// Verify it's properly in the future (past challenge finality requirement)
			minRequired := tt.currentHeight + tt.challengeFinality.Int64()
			if resultInt <= minRequired {
				t.Errorf("adjustNextProveAt() = %d, should be > %d (current + finality)",
					resultInt, minRequired)
			}

			// Verify it's exactly 1 epoch after a window boundary
			windowSize := tt.challengeWindow.Int64()
			if (resultInt-1)%windowSize != 0 {
				t.Errorf("adjustNextProveAt() = %d, should be 1 epoch after window boundary (multiple of %d)",
					resultInt, windowSize)
			}

			t.Logf("%s: currentHeight=%d -> nextProveAt=%d (1 epoch after window boundary)",
				tt.description, tt.currentHeight, resultInt)
		})
	}
}
