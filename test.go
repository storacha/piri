package main

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/storacha/piri/pkg/pdp/proof"
)

func main() {
	fmt.Println(abi.PaddedPieceSize(proof.MaxMemtreeSize).Unpadded())
}
