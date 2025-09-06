//go:generate bash ../../../../scripts/generate-contracts.sh

package contract

// This file exists solely to provide a go:generate hook for contract generation.
// It contains no actual Go code and serves only to trigger contract regeneration.
//
// To regenerate contract bindings, run:
//   go generate ./pkg/pdp/service/contract/
//
// Or to regenerate all contracts in the project:
//   go generate ./...
