package pdp

import (
	"github.com/storacha/piri/pkg/pdp/piece"
	"github.com/storacha/piri/pkg/pdp/types"
)

type PDP interface {
	API() types.PieceAPI
	CommpCalculate() piece.Calculator
}
