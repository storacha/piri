package client

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/client"
)

type Config struct {
	KeyFile string
	Proof   string
	NodeDID string
	NodeURL string
}

func New(cfg Config) (*client.Client, error) {
	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	proofFile, err := os.Open(cfg.Proof)
	if err != nil {
		return nil, fmt.Errorf("opening delegation car file: %w", err)
	}
	proofData, err := io.ReadAll(proofFile)
	if err != nil {
		return nil, fmt.Errorf("reading delegation car file: %w", err)
	}
	proof, err := delegation.Extract(proofData)
	if err != nil {
		return nil, fmt.Errorf("extracting storage proof: %w", err)
	}
	nodeDID, err := did.Parse(cfg.NodeDID)
	if err != nil {
		return nil, fmt.Errorf("parsing node did: %w", err)
	}
	nodeURL, err := url.Parse(cfg.NodeURL)
	if err != nil {
		return nil, fmt.Errorf("parsing node url: %w", err)
	}
	c, err := client.NewClient(client.Config{
		ID:             id,
		StorageNodeID:  nodeDID,
		StorageNodeURL: *nodeURL,
		StorageProof:   delegation.FromDelegation(proof),
	})
	if err != nil {
		return nil, fmt.Errorf("setting up client: %w", err)
	}
	return c, nil
}
