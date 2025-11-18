package service

import (
	"context"
)

type ConfiguredProofSetProvider struct {
	ID uint64
}

func (c *ConfiguredProofSetProvider) ProofSetID(ctx context.Context) (uint64, error) {
	return c.ID, nil
}
