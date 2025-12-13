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
			name:              "exact scenario from logs",
			currentHeight:     2685164,
			challengeFinality: big.NewInt(2),
			challengeWindow:   big.NewInt(30),
			expected:          2685181, // minRequired=2685166, next window=2685180, result=2685181 (2685180+1)
			description:       "Real scenario should produce predictable window placement",
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
			name:              "smart contract realistic values",
			currentHeight:     1000000,
			challengeFinality: big.NewInt(2),  // MinConfidence from watcher_eth.go
			challengeWindow:   big.NewInt(30), // Common challenge window from tests
			expected:          1000021,        // minRequired=1000002, windowStart=999990+30=1000020, result=1000021 (1000020+1)
			description:       "Realistic smart contract values with challenge window 30 and finality 2",
		},
		{
			name:              "proving period scenario",
			currentHeight:     500000,
			challengeFinality: big.NewInt(2),
			challengeWindow:   big.NewInt(60), // As requested - proving period of 60
			expected:          500041,         // minRequired=500002, next window=500040, result=500041 (500040+1)
			description:       "Scenario with proving period/challenge window of 60 epochs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
