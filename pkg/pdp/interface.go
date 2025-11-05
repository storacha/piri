package pdp

import (
	"github.com/storacha/piri/pkg/pdp/comper"
	"github.com/storacha/piri/pkg/pdp/types"
)

type PDP interface {
	API() types.PieceAPI
	CommpCalculator() comper.Calculator
}
