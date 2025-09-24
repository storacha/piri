// Code generated - DO NOT EDIT.
package bindings

import (
	"math/big"
)

// CidsCid is an auto generated low-level Go binding around an user-defined struct.
type CidsCid struct {
	Data []byte
}

// Common types used across contracts
type IPDPTypesProof struct {
	Leaf  [32]byte
	Proof [][32]byte
}

type IPDPTypesPieceIdAndOffset struct {
	PieceId *big.Int
	Offset  *big.Int
}
