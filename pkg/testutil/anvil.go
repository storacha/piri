// Package testutil provides testing utilities for integration tests.
package testutil

import (
	"github.com/ethereum/go-ethereum/rpc"
)

// AnvilClient wraps RPC calls to control Anvil chain behavior.
// Anvil is a local Ethereum development environment that supports
// special RPC methods for test control.
type AnvilClient struct {
	rpcClient *rpc.Client
}

// NewAnvilClient creates a new AnvilClient connected to the given RPC URL.
func NewAnvilClient(rpcURL string) (*AnvilClient, error) {
	client, err := rpc.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	return &AnvilClient{rpcClient: client}, nil
}

// Close closes the underlying RPC connection.
func (c *AnvilClient) Close() {
	c.rpcClient.Close()
}

// MineBlock triggers Anvil to mine a single block.
// This is useful for advancing the chain state in tests.
func (c *AnvilClient) MineBlock() error {
	return c.rpcClient.Call(nil, "evm_mine")
}

// MineBlocks mines n blocks sequentially.
// Each block is mined individually to ensure proper state transitions.
func (c *AnvilClient) MineBlocks(n int) error {
	for i := 0; i < n; i++ {
		if err := c.MineBlock(); err != nil {
			return err
		}
	}
	return nil
}

// SetAutoMine enables or disables automatic block mining.
// When enabled, Anvil mines a block after every transaction.
// When disabled, transactions remain pending until MineBlock is called.
func (c *AnvilClient) SetAutoMine(enabled bool) error {
	return c.rpcClient.Call(nil, "evm_setAutomine", enabled)
}

// SetBlockTimestampInterval sets the interval between block timestamps.
// This affects the timestamp used when mining new blocks.
func (c *AnvilClient) SetBlockTimestampInterval(seconds uint64) error {
	return c.rpcClient.Call(nil, "anvil_setBlockTimestampInterval", seconds)
}
