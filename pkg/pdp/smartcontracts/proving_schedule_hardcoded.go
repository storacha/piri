package smartcontracts

import (
	"context"
	"math/big"
)

const (
	// NB: challenge finality of the PDPVerifier on calibnet is set to 1

	// Conservative values - suitable for production with higher challengeFinality
	HardcodedMaxProvingPeriod   = 60
	HardcodedChallengeWindow    = 30
	HardcodedChallengesPerProof = 5

	// Fast testing values - suitable for testing with challengeFinality = 1
	// Provides rapid proving cycles (~5 minutes on Filecoin) for development
	FastTestMaxProvingPeriod   = 10
	FastTestChallengeWindow    = 5
	FastTestChallengesPerProof = 5
)

// HardcodedProvingSchedule implements ProvingScheduleProvider using hardcoded values.
// This mimics the behavior of a service contract without requiring actual contract deployment.
type HardcodedProvingSchedule struct {
	chain ChainAPI
}

// NewHardcodedProvingSchedule creates a new hardcoded proving schedule provider
func NewHardcodedProvingSchedule(chain ChainAPI) *HardcodedProvingSchedule {
	return &HardcodedProvingSchedule{
		chain: chain,
	}
}

// GetPDPConfig returns hardcoded proving configuration parameters
func (h *HardcodedProvingSchedule) GetPDPConfig(ctx context.Context) (PDPConfig, error) {
	// Get current chain height to calculate InitChallengeWindowStart
	ts, err := h.chain.ChainHead(ctx)
	if err != nil {
		return PDPConfig{}, err
	}

	// Use fast testing values for rapid iteration
	maxProvingPeriod := HardcodedMaxProvingPeriod
	challengeWindow := HardcodedChallengeWindow

	// InitChallengeWindowStart is current height + maxProvingPeriod
	// This gives the dataset one full proving period before the first challenge
	initWindowStart := int64(ts.Height()) + int64(maxProvingPeriod)

	return PDPConfig{
		MaxProvingPeriod:         uint64(maxProvingPeriod),
		ChallengeWindow:          big.NewInt(int64(challengeWindow)),
		ChallengesPerProof:       big.NewInt(HardcodedChallengesPerProof),
		InitChallengeWindowStart: big.NewInt(initWindowStart),
	}, nil
}

// NextPDPChallengeWindowStart calculates the next challenge window start epoch.
// This mimics the service contract's calculation logic.
func (h *HardcodedProvingSchedule) NextPDPChallengeWindowStart(ctx context.Context, setId *big.Int) (*big.Int, error) {
	// Get current chain height
	ts, err := h.chain.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	currentHeight := int64(ts.Height())

	// Use fast testing values for rapid iteration
	maxProvingPeriod := HardcodedMaxProvingPeriod
	challengeWindow := HardcodedChallengeWindow

	// Calculate next window start:
	// - Add a full proving period to current height
	// - Subtract half the challenge window to land in the middle of the window
	// This gives a buffer so proofs can be submitted within the challenge window
	nextWindowStart := currentHeight + int64(maxProvingPeriod) - (int64(challengeWindow) / 2)

	return big.NewInt(nextWindowStart), nil
}
