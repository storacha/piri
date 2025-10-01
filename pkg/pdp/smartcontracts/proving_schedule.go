package smartcontracts

import (
	"context"
	"math/big"

	chaintypes "github.com/filecoin-project/lotus/chain/types"
)

// ChainAPI interface for chain operations needed by proving schedule
type ChainAPI interface {
	ChainHead(context.Context) (*chaintypes.TipSet, error)
}

// ProvingScheduleProvider abstracts proving schedule operations.
// This can be backed by hardcoded values or a real service contract.
type ProvingScheduleProvider interface {
	// GetPDPConfig returns the proving configuration parameters
	GetPDPConfig(ctx context.Context) (PDPConfig, error)

	// NextPDPChallengeWindowStart calculates the next challenge window start epoch
	NextPDPChallengeWindowStart(ctx context.Context, setId *big.Int) (*big.Int, error)
}

// PDPConfig holds proving period configuration
type PDPConfig struct {
	MaxProvingPeriod         uint64
	ChallengeWindow          *big.Int
	ChallengesPerProof       *big.Int
	InitChallengeWindowStart *big.Int
}
