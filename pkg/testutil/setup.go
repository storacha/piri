package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"

	piriclient "github.com/storacha/piri/pkg/client"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
)

// TestSetup provides a fluent builder for common test prerequisites.
// It encapsulates the common setup steps needed for PDP integration tests:
// registering a provider, approving it, and creating a proof set.
type TestSetup struct {
	T        testing.TB
	Harness  *Harness
	NodeInfo *NodeInfo
	API      types.API
	Ctx      context.Context

	// Provider state
	ProviderID     uint64
	ProviderStatus types.GetProviderStatusResults
	ProviderRegTx  common.Hash

	// Proof set state
	ProofSetID uint64
	ProofSetTx common.Hash
}

// NewTestSetup creates a new test setup builder.
func NewTestSetup(t testing.TB, harness *Harness, nodeInfo *NodeInfo) *TestSetup {
	return &TestSetup{
		T:        t,
		Harness:  harness,
		NodeInfo: nodeInfo,
		API:      nodeInfo.API,
		Ctx:      t.Context(),
	}
}

// RegisterProvider registers a new provider and waits for confirmation.
// It stores the provider ID and status in the TestSetup for later use.
func (s *TestSetup) RegisterProvider(name, description string) *TestSetup {
	result, err := s.API.RegisterProvider(s.Ctx, types.RegisterProviderParams{
		Name:        name,
		Description: description,
	})
	require.NoError(s.T, err, "RegisterProvider should succeed")

	s.ProviderRegTx = result.TransactionHash
	s.Harness.Operator.WaitForTxConfirmation(result.TransactionHash, 60*time.Second)
	require.NoError(s.T, s.Harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err := s.API.GetProviderStatus(s.Ctx)
	require.NoError(s.T, err)
	require.True(s.T, status.IsRegistered, "Provider should be registered")
	require.Greater(s.T, status.ID, uint64(0), "Provider ID should be assigned")

	s.ProviderID = status.ID
	s.ProviderStatus = status

	return s
}

// ApproveProvider approves the registered provider.
// Must be called after RegisterProvider.
func (s *TestSetup) ApproveProvider() *TestSetup {
	require.Greater(s.T, s.ProviderID, uint64(0), "Must register provider before approving")

	s.Harness.Operator.ApproveProvider(s.ProviderID)
	require.NoError(s.T, s.Harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err := s.API.GetProviderStatus(s.Ctx)
	require.NoError(s.T, err)
	require.True(s.T, status.IsApproved, "Provider should be approved")

	s.ProviderStatus = status

	return s
}

// CreateProofSet creates a proof set and waits for it to be ready.
// Must be called after ApproveProvider.
func (s *TestSetup) CreateProofSet() *TestSetup {
	require.True(s.T, s.ProviderStatus.IsApproved, "Must approve provider before creating proof set")

	txHash, err := s.API.CreateProofSet(s.Ctx)
	require.NoError(s.T, err, "CreateProofSet should succeed")

	s.ProofSetTx = txHash
	s.Harness.Operator.WaitForTxConfirmation(txHash, 60*time.Second)
	require.NoError(s.T, s.Harness.Chain.MineBlocks(tasks.MinConfidence))
	time.Sleep(6 * time.Second) // Wait for watcher to process

	psStatus, err := s.API.GetProofSetStatus(s.Ctx, txHash)
	require.NoError(s.T, err)
	require.True(s.T, psStatus.Created, "Proof set should be created")

	s.ProofSetID = psStatus.ID

	return s
}

// NewBlobClient creates a UCAN client for blob operations.
// clientSigner is the identity of the client (e.g., a different signer than the node).
// nodeInfo provides the node's identity and URL for creating the delegation.
func (s *TestSetup) NewBlobClient(clientSigner principal.Signer, nodeInfo *NodeInfo) *piriclient.Client {
	// Create delegation from node to client for blob operations
	d, err := delegation.Delegate(
		nodeInfo.Signer, // issuer: node's identity
		clientSigner,    // audience: client's identity
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(blob.AllocateAbility, nodeInfo.Signer.DID().String(), ucan.NoCaveats{}),
			ucan.NewCapability(blob.AcceptAbility, nodeInfo.Signer.DID().String(), ucan.NoCaveats{}),
			ucan.NewCapability(pdp.InfoAbility, nodeInfo.Signer.DID().String(), ucan.NoCaveats{}),
		},
		delegation.WithNoExpiration(),
	)
	require.NoError(s.T, err, "Failed to create delegation")

	client, err := piriclient.NewClient(piriclient.Config{
		ID:             clientSigner,
		StorageNodeID:  nodeInfo.Signer,
		StorageNodeURL: nodeInfo.URL,
		StorageProof:   delegation.FromDelegation(d),
	})
	require.NoError(s.T, err, "Failed to create client")

	return client
}

// WaitForRoots polls until roots are added to the proof set.
// Returns the roots once available.
func (s *TestSetup) WaitForRoots(timeout time.Duration) []types.RootEntry {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ps, err := s.API.GetProofSet(s.Ctx, s.ProofSetID)
		if err == nil && len(ps.Roots) > 0 {
			return ps.Roots
		}
		_ = s.Harness.Chain.MineBlocks(1)
		time.Sleep(500 * time.Millisecond)
	}
	s.T.Fatalf("timeout waiting for roots to be added to proof set %d", s.ProofSetID)
	return nil
}

// WaitForProofSubmission polls the database until a proof is submitted.
// A proof is considered submitted when ChallengeRequestMsgHash becomes NULL.
func (s *TestSetup) WaitForProofSubmission(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ps models.PDPProofSet
		err := s.NodeInfo.EngineDB.Where("id = ?", s.ProofSetID).First(&ps).Error
		if err == nil && ps.ChallengeRequestMsgHash == nil && ps.ProveAtEpoch != nil {
			return // Proof submitted - msg hash cleared
		}
		_ = s.Harness.Chain.MineBlocks(1)
		time.Sleep(500 * time.Millisecond)
	}
	s.T.Fatalf("timeout waiting for proof submission for proof set %d", s.ProofSetID)
}

// WaitForProvingPeriodAdvance polls until ProveAtEpoch increases beyond previousEpoch.
// Returns the new ProveAtEpoch.
func (s *TestSetup) WaitForProvingPeriodAdvance(previousEpoch int64, timeout time.Duration) int64 {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ps models.PDPProofSet
		err := s.NodeInfo.EngineDB.Where("id = ?", s.ProofSetID).First(&ps).Error
		if err == nil && ps.ProveAtEpoch != nil && *ps.ProveAtEpoch > previousEpoch {
			return *ps.ProveAtEpoch
		}
		_ = s.Harness.Chain.MineBlocks(1)
		time.Sleep(500 * time.Millisecond)
	}
	s.T.Fatalf("timeout waiting for proving period to advance beyond epoch %d", previousEpoch)
	return 0
}

// GetProveAtEpoch returns the current ProveAtEpoch from the database.
func (s *TestSetup) GetProveAtEpoch() int64 {
	var ps models.PDPProofSet
	err := s.NodeInfo.EngineDB.Where("id = ?", s.ProofSetID).First(&ps).Error
	require.NoError(s.T, err)
	require.NotNil(s.T, ps.ProveAtEpoch, "ProveAtEpoch should not be nil")
	return *ps.ProveAtEpoch
}
