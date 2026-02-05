package localdev

import (
	_ "embed"
)

// AnvilStateJSON contains the embedded Anvil state file for fast container startup.
// This state includes pre-deployed contracts and initial chain state.
//
//go:embed anvil-state.json
var AnvilStateJSON []byte

// DeployedAddressesJSON contains the embedded deployed contract addresses.
// This corresponds to the contracts deployed in AnvilStateJSON.
//
//go:embed deployed-addresses.json
var DeployedAddressesJSON []byte
