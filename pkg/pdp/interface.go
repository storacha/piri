package pdp

import (
	"github.com/storacha/piri/pkg/pdp/aggregation/commp"
	"github.com/storacha/piri/pkg/pdp/types"
)

type PDP interface {
	API() types.PieceAPI
	CommpCalculate() commp.Calculator
}
