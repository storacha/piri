package pdp

import (
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/types"
)

type PDP interface {
	API() types.PieceAPI
	CommpCalculate() aggregator.CommpQueue
}
