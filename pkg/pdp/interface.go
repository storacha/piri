package pdp

import (
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
)

type PDP interface {
	PieceAdder() pieceadder.PieceAdder
	PieceFinder() piecefinder.PieceFinder
	Aggregator() aggregator.Aggregator
}
