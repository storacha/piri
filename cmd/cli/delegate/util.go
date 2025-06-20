package delegate

import (
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/ucan"
)

// these methods were ported from https://github.com/storacha/go-mkdelegation/blob/main/pkg/delegation/delegation.go

func MakeDelegation(issuer ucan.Signer, audience ucan.Principal, capabilities []string, opts ...delegation.Option) (delegation.Delegation, error) {
	uc := make([]ucan.Capability[ucan.NoCaveats], len(capabilities))
	for i, capability := range capabilities {
		uc[i] = ucan.NewCapability(
			capability,
			issuer.DID().String(),
			ucan.NoCaveats{},
		)
	}

	return delegation.Delegate(
		issuer,
		audience,
		uc,
		opts...,
	)
}

// FormatDelegation takes a delegation archive from a read and returns a multibase-base64-encoded CIDv1 with
// embedded CAR data.
func FormatDelegation(d io.Reader) (string, error) {
	db, err := io.ReadAll(d)
	if err != nil {
		return "", fmt.Errorf("failed to read delegation: %w", err)
	}

	return FormatDelegationBytes(db)
}

// FormatDelegationBytes takes a delegation archive in byte form and returns a multibase-base64-encoded CIDv1 with
// embedded CAR data.
func FormatDelegationBytes(archive []byte) (string, error) {
	// Create identity digest of the archive
	// The identity hash function (0x00) simply returns the input data as the hash
	mh, err := multihash.Sum(archive, multihash.IDENTITY, -1)
	if err != nil {
		return "", fmt.Errorf("failed to create identity hash: %w", err)
	}

	// Create a CID (Content IDentifier) with codec 0x0202 (CAR format)
	// The 0x0202 codec is defined in the multicodec table for Content Addressable aRchives (CAR)
	link := cid.NewCidV1(uint64(multicodec.Car), mh)

	// Convert the CID to base64 encoding
	str, err := link.StringOfBase(multibase.Base64)
	if err != nil {
		return "", fmt.Errorf("failed to encode CID to base64: %w", err)
	}

	return str, nil
}
