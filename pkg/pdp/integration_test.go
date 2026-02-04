package pdp_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"net/http"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	libstorachatestutil "github.com/storacha/go-libstoracha/testutil"
	ucsha256 "github.com/storacha/go-ucanto/core/ipld/hash/sha256"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/testutil"
)

// TestPDPProviderRegistration tests provider registration and approval flow.
// This verifies:
// 1. Provider can register with the registry
// 2. Contract owner can approve the provider
// 3. Provider status reflects registration and approval state
func TestPDPProviderRegistration(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Register provider
	result, err := api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "test-provider",
		Description: "Test provider for integration testing",
	})
	require.NoError(t, err, "RegisterProvider should succeed")
	t.Logf("RegisterProvider tx: %s", result.TransactionHash)

	// Wait for registration transaction to be confirmed
	harness.Operator.WaitForTxConfirmation(result.TransactionHash, 60*time.Second)

	// Mine blocks to allow watcher to process the event
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	// Check provider status - should be registered but not yet approved
	status, err := api.GetProviderStatus(ctx)
	require.NoError(t, err, "GetProviderStatus should succeed")
	require.True(t, status.IsRegistered, "Provider should be registered")
	require.False(t, status.IsApproved, "Provider should not yet be approved")
	require.Greater(t, status.ID, uint64(0), "Provider should have a valid ID")

	t.Logf("Provider registered with ID: %d", status.ID)

	// Approve provider using deployer (contract owner) key
	harness.Operator.ApproveProvider(status.ID)

	// Mine blocks for approval to propagate
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	// Check provider status again - should now be approved
	status, err = api.GetProviderStatus(ctx)
	require.NoError(t, err, "GetProviderStatus should succeed after approval")
	require.True(t, status.IsRegistered, "Provider should still be registered")
	require.True(t, status.IsApproved, "Provider should now be approved")

	t.Logf("Provider approved successfully")
}

// TestPDPProviderAlreadyRegistered verifies that registering an already-registered provider fails.
func TestPDPProviderAlreadyRegistered(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Register provider first time
	result, err := api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "test-provider",
		Description: "First registration",
	})
	require.NoError(t, err)
	harness.Operator.WaitForTxConfirmation(result.TransactionHash, 60*time.Second)

	// Mine blocks to process
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	// Verify registration succeeded
	status, err := api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsRegistered)

	// Try to register again - should fail
	_, err = api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "test-provider-2",
		Description: "Second registration",
	})
	require.Error(t, err, "Second registration should fail")
	t.Logf("Expected error on second registration: %v", err)
}

// TestPDPCreateProofSet tests creating a proof set after provider registration.
func TestPDPCreateProofSet(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Register and approve provider first
	result, err := api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "test-provider",
		Description: "Test provider",
	})
	require.NoError(t, err)
	harness.Operator.WaitForTxConfirmation(result.TransactionHash, 60*time.Second)

	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err := api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsRegistered)

	// Approve provider
	harness.Operator.ApproveProvider(status.ID)

	// Mine blocks for approval to propagate
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	// Verify approval
	status, err = api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsApproved, "Provider must be approved before creating proof set")

	// Create proof set
	proofSetTxHash, err := api.CreateProofSet(ctx)
	require.NoError(t, err, "CreateProofSet should succeed")
	t.Logf("CreateProofSet tx: %s", proofSetTxHash.Hex())

	// Wait for proof set creation transaction to be confirmed
	harness.Operator.WaitForTxConfirmation(proofSetTxHash, 60*time.Second)

	// Mine blocks for watcher to process the event
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))
	time.Sleep(6 * time.Second)

	// Verify proof set was created
	psStatus, err := api.GetProofSetStatus(ctx, proofSetTxHash)
	require.NoError(t, err, "GetProofSetStatus should succeed")
	require.True(t, psStatus.Created, "Proof set should be created")

	proofSet, err := api.GetProofSet(ctx, psStatus.ID)
	require.NoError(t, err, "GetProofSet should succeed")

	t.Logf("Created proof set: ID=%d", proofSet.ID)
}

// TestPDPCreateProofSetNotApproved tests that creating a proof set fails if provider is not approved.
func TestPDPCreateProofSetNotApproved(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Register provider but do NOT approve
	result, err := api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "test-provider",
		Description: "Test provider",
	})
	require.NoError(t, err)
	harness.Operator.WaitForTxConfirmation(result.TransactionHash, 60*time.Second)

	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err := api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsRegistered)
	require.False(t, status.IsApproved, "Provider should not be approved yet")

	// Try to create proof set without approval - should fail
	_, err = api.CreateProofSet(ctx)
	require.Error(t, err, "CreateProofSet should fail without provider approval")
	t.Logf("Expected error for unapproved provider: %v", err)
}

// TestPDPPieceUpload tests allocating and uploading a piece.
func TestPDPPieceUpload(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Create test data
	testData := []byte("Hello, this is test data for PDP piece upload testing!")
	hash := sha256.Sum256(testData)

	// Create multihash for the data
	mh, err := multihash.Encode(hash[:], multihash.SHA2_256)
	require.NoError(t, err)

	// Allocate piece
	allocation := types.PieceAllocation{
		Piece: types.Piece{
			Name: multicodec.Sha2_256.String(),
			Hash: mh,
			Size: int64(len(testData)),
		},
		Notify: nil,
	}

	allocated, err := api.AllocatePiece(ctx, allocation)
	require.NoError(t, err, "AllocatePiece should succeed")
	require.True(t, allocated.Allocated, "Piece should be allocated (not already present)")
	require.NotEmpty(t, allocated.UploadID, "Should have a valid upload ID")

	t.Logf("Allocated piece with upload ID: %s", allocated.UploadID)

	// Upload the piece
	upload := types.PieceUpload{
		ID:   allocated.UploadID,
		Data: bytes.NewReader(testData),
	}

	err = api.UploadPiece(ctx, upload)
	require.NoError(t, err, "UploadPiece should succeed")

	t.Logf("Piece uploaded successfully")

	// Verify we can check if piece exists
	has, err := api.Has(ctx, mh)
	require.NoError(t, err, "Has should succeed")
	require.True(t, has, "Piece should exist after upload")

	// Trying to allocate same piece should return Allocated=false (already exists)
	allocated2, err := api.AllocatePiece(ctx, allocation)
	require.NoError(t, err, "Second AllocatePiece should succeed")
	require.False(t, allocated2.Allocated, "Piece should not need allocation (already present)")
}

// TestPDPCommPCalculation tests calculating CommP for uploaded piece.
func TestPDPCommPCalculation(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	// Create test data - must be padded to power of 2 for CommP
	// Using a small piece for testing
	testData := make([]byte, 1024) // 1KB piece
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	hash := sha256.Sum256(testData)

	// Create multihash for the data
	mh, err := multihash.Encode(hash[:], multihash.SHA2_256)
	require.NoError(t, err)

	// Allocate and upload piece
	allocation := types.PieceAllocation{
		Piece: types.Piece{
			Name: multicodec.Sha2_256.String(),
			Hash: mh,
			Size: int64(len(testData)),
		},
	}

	allocated, err := api.AllocatePiece(ctx, allocation)
	require.NoError(t, err)
	require.True(t, allocated.Allocated)

	err = api.UploadPiece(ctx, types.PieceUpload{
		ID:   allocated.UploadID,
		Data: bytes.NewReader(testData),
	})
	require.NoError(t, err)

	// Calculate CommP
	commPResp, err := api.CalculateCommP(ctx, mh)
	require.NoError(t, err, "CalculateCommP should succeed")
	require.True(t, commPResp.PieceCID.Defined(), "PieceCID should be defined")
	require.Greater(t, commPResp.RawSize, int64(0), "RawSize should be greater than 0")

	t.Logf("CommP calculated: %s, RawSize: %d, PaddedSize: %d", commPResp.PieceCID.String(), commPResp.RawSize, commPResp.PaddedSize)

	// Calculate again - should return cached result
	commPResp2, err := api.CalculateCommP(ctx, mh)
	require.NoError(t, err)
	require.Equal(t, commPResp.PieceCID.String(), commPResp2.PieceCID.String(), "Cached CommP should match")
}

// TestPDPFullLifecycle tests the complete end-to-end flow:
// 1. Provider registration and approval
// 2. Proof set creation
// 3. Piece upload and CommP calculation
func TestPDPFullLifecycle(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)
	nodeInfo, stop := testutil.NewNode(t, harness.Container, libstorachatestutil.Alice)
	t.Cleanup(stop)
	ctx := t.Context()
	api := nodeInfo.API

	t.Log("=== Step 1: Provider Registration ===")

	// Register provider
	regResult, err := api.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        "lifecycle-provider",
		Description: "Full lifecycle test provider",
	})
	require.NoError(t, err)
	harness.Operator.WaitForTxConfirmation(regResult.TransactionHash, 60*time.Second)
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err := api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsRegistered, "Provider should be registered")
	t.Logf("Provider registered with ID: %d", status.ID)

	t.Log("=== Step 2: Provider Approval ===")

	// Approve provider
	harness.Operator.ApproveProvider(status.ID)

	// Mine blocks for approval to propagate
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))

	status, err = api.GetProviderStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.IsApproved, "Provider should be approved")
	t.Log("Provider approved")

	t.Log("=== Step 3: Proof Set Creation ===")

	// Create proof set
	proofSetTxHash, err := api.CreateProofSet(ctx)
	require.NoError(t, err)
	harness.Operator.WaitForTxConfirmation(proofSetTxHash, 60*time.Second)

	// Mine blocks for watcher to process the event
	require.NoError(t, harness.Chain.MineBlocks(tasks.MinConfidence))
	time.Sleep(6 * time.Second)

	// Verify proof set
	psStatus, err := api.GetProofSetStatus(ctx, proofSetTxHash)
	require.NoError(t, err)
	require.True(t, psStatus.Created, "Proof set should be created")

	proofSet, err := api.GetProofSet(ctx, psStatus.ID)
	require.NoError(t, err)
	t.Logf("Proof set created: ID=%d", proofSet.ID)

	t.Log("=== Step 4: Piece Upload ===")

	// Upload a piece
	testData := make([]byte, 2048) // 2KB piece
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	hash := sha256.Sum256(testData)
	mh, err := multihash.Encode(hash[:], multihash.SHA2_256)
	require.NoError(t, err)

	allocation := types.PieceAllocation{
		Piece: types.Piece{
			Name: multicodec.Sha2_256.String(),
			Hash: mh,
			Size: int64(len(testData)),
		},
	}

	allocated, err := api.AllocatePiece(ctx, allocation)
	require.NoError(t, err)
	require.True(t, allocated.Allocated)

	err = api.UploadPiece(ctx, types.PieceUpload{
		ID:   allocated.UploadID,
		Data: bytes.NewReader(testData),
	})
	require.NoError(t, err)
	t.Logf("Piece uploaded successfully")

	t.Log("=== Step 5: CommP Calculation ===")

	// Calculate CommP for the piece
	commPResp, err := api.CalculateCommP(ctx, mh)
	require.NoError(t, err)
	require.True(t, commPResp.PieceCID.Defined(), "PieceCID should be defined")
	t.Logf("CommP calculated: %s, RawSize: %d", commPResp.PieceCID.String(), commPResp.RawSize)

	t.Log("=== Full Lifecycle Complete ===")
	t.Logf("Summary: Provider ID=%d, ProofSet ID=%d, Piece CommP=%s",
		status.ID, proofSet.ID, commPResp.PieceCID.String())
}

func TestBlobUploadFlow(t *testing.T) {
	t.Parallel()
	harness := testutil.NewHarness(t)

	// Explicit signers - node and client use different identities
	nodeSigner := libstorachatestutil.Alice
	clientSigner := libstorachatestutil.Bob

	nodeInfo, stop := testutil.NewNode(t, harness.Container, nodeSigner)
	t.Cleanup(stop)

	setup := testutil.NewTestSetup(t, harness, nodeInfo).
		RegisterProvider("testing", "testing").
		ApproveProvider().
		CreateProofSet()

	// Create UCAN client with explicit client signer
	client := setup.NewBlobClient(clientSigner, nodeInfo)

	ctx := t.Context()

	// Create test blob sized such that its aggregated
	blobData := make([]byte, 200*1024*1024)
	_, err := rand.Read(blobData)
	require.NoError(t, err)

	digest, err := ucsha256.Hasher.Sum(blobData)
	require.NoError(t, err)

	spaceDID := clientSigner.DID() // Use client's DID as space
	cause := cidlink.Link{Cid: cid.NewCidV1(cid.Raw, digest.Bytes())}

	// Step 1: BlobAllocate
	address, err := client.BlobAllocate(ctx, spaceDID, digest.Bytes(), uint64(len(blobData)), cause)
	require.NoError(t, err, "BlobAllocate should succeed")
	t.Logf("BlobAllocate succeeded, address: %v", address)

	// Step 2: HTTP PUT (if address is not nil - blob not already present)
	if address != nil {
		req, err := http.NewRequest(http.MethodPut, address.URL.String(), bytes.NewReader(blobData))
		require.NoError(t, err)
		req.Header = address.Headers
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.True(t, res.StatusCode >= 200 && res.StatusCode < 300, "HTTP PUT should succeed, got status %d", res.StatusCode)
		res.Body.Close()
		t.Logf("HTTP PUT succeeded")
	}

	// Step 3: BlobAccept
	result, err := client.BlobAccept(ctx, spaceDID, digest.Bytes(), uint64(len(blobData)), cause)
	require.NoError(t, err, "BlobAccept should succeed")
	require.NotNil(t, result.LocationCommitment, "Should have location commitment")
	t.Logf("BlobAccept succeeded, location commitment: %+v", result.LocationCommitment)

	// === ASSERTION 1: Roots Added ===
	t.Log("Waiting for roots to be added to proof set...")
	roots := setup.WaitForRoots(2 * time.Minute)
	require.Greater(t, len(roots), 0, "Roots should be added after blob upload")
	t.Logf("Roots added: %d entries", len(roots))

	// Get initial proving epoch from database
	initialProveAtEpoch := setup.GetProveAtEpoch()
	t.Logf("Initial ProveAtEpoch: %d", initialProveAtEpoch)

	// === ASSERTION 2: Proof Submitted ===
	t.Log("Waiting for proof submission...")
	setup.WaitForProofSubmission(3 * time.Minute)
	t.Log("Proof submitted successfully")

	// === ASSERTION 3: Proving Period Advances ===
	t.Log("Waiting for proving period to advance...")
	newProveAtEpoch := setup.WaitForProvingPeriodAdvance(initialProveAtEpoch, 2*time.Minute)
	require.Greater(t, newProveAtEpoch, initialProveAtEpoch, "Proving period should advance")
	t.Logf("Proving period advanced: %d → %d", initialProveAtEpoch, newProveAtEpoch)

	// === Second Cycle: Verify stability ===
	t.Log("Verifying second proving cycle...")
	setup.WaitForProofSubmission(3 * time.Minute)
	t.Log("Second proof submitted successfully")
	finalProveAtEpoch := setup.WaitForProvingPeriodAdvance(newProveAtEpoch, 2*time.Minute)
	require.Greater(t, finalProveAtEpoch, newProveAtEpoch, "Second proving period should advance")
	t.Logf("Second cycle complete: %d → %d", newProveAtEpoch, finalProveAtEpoch)
}
