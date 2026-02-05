package testutil

import (
	"crypto/ecdsa"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func HexToECDSA(t testing.TB, privateKey string) *ecdsa.PrivateKey {
	out, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	return out
}
