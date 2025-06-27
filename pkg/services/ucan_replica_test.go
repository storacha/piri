package services_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	spaceblob "github.com/storacha/go-libstoracha/capabilities/space/blob"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/car"
	"github.com/storacha/go-ucanto/core/dag/blockstore"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/ipld"
	"github.com/storacha/go-ucanto/core/message"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	"github.com/storacha/go-ucanto/core/result/ok"
	"github.com/storacha/go-ucanto/did"
	ucanto "github.com/storacha/go-ucanto/ucan"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/datastores"
	"github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/services"
	"github.com/storacha/piri/pkg/services/config"
	"github.com/storacha/piri/pkg/services/replicator"
	servicetypes "github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/services/ucan"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

// TestReplicaAllocateTransfer validates the full replica allocation flow in the UCAN server,
// ensuring that invocations are correctly constructed and executed, and that the simulated endpoints
// interact as expected. A lightweight HTTP server (on port 8080) is used to simulate external endpoints:
//   - "/get": Represents the source node that returns the original blob data.
//   - "/put": Emulates the replica node that accepts and stores the blob.
//   - "/upload-service": Acts as the upload service by decoding a CAR payload and triggering a transfer receipt.
//
// This test covers three scenarios:
//  1. **NoExistingAllocationNoData:** No previous allocation or stored data exists, so the full blob is transferred.
//  2. **ExistingAllocationNoData:** An allocation record is present (indicating reserved space) but the blob data is not yet stored,
//     resulting in no additional data, but involving a transfer
//  3. **ExistingAllocationAndData:** Both an allocation record and the blob data are already present; although a transfer receipt is still produced,
//     no redundant data transfer should occur.
func TestReplicaAllocateTransfer(t *testing.T) {
	testCases := []struct {
		name                  string
		hasExistingAllocation bool
		hasExistingData       bool
		expectedTransferSize  uint64
	}{
		{
			name:                  "NoExistingAllocationNoData",
			hasExistingAllocation: false,
			hasExistingData:       false,
		},
		{
			name:                  "ExistingAllocationNoData",
			hasExistingAllocation: true,
			hasExistingData:       false,
		},
		{
			name:                  "ExistingAllocationAndData",
			hasExistingAllocation: true,
			hasExistingData:       true,
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// we expect each test to run in 10 seconds or less.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// Common setup: random DID, random data, etc.
			expectedSpace := testutil.RandomDID(t)
			expectedSize := uint64(rand.IntN(32) + 1)
			expectedData := testutil.RandomBytes(t, int(expectedSize))
			expectedDigest := testutil.Must(
				multihash.Sum(expectedData, multihash.SHA2_256, -1),
			)(t)
			replicas := uint(1)
			serverAddr := ":8080"
			sourcePath, sinkPath, uploadServicePath := "get", "put", "upload-service"

			// Spin up storage service, using injected values for testing.
			locationURL, uploadServiceURL, fakeBlobPresigner := setupURLs(t, serverAddr, sourcePath, sinkPath, uploadServicePath)
			svc, srv := setupService(t, fakeBlobPresigner, uploadServiceURL)
			fakeServer, transferOkChan := startTestHTTPServer(
				ctx, t, expectedDigest, expectedData, svc,
				serverAddr, sourcePath, sinkPath, uploadServicePath,
			)
			t.Cleanup(func() {
				fakeServer.Close()
				cancel()
			})

			// Build UCAN connection
			conn := testutil.Must(client.NewConnection(testutil.Service, srv))(t)

			// Build UCAN delegation + location claim + replicate invocation
			// required ability's for blob replicate
			prf := buildDelegationProof(t)
			// location claim and blob replicate invocation, simulating an upload-service
			lcd, expectedLocationCaveats := buildLocationClaim(t, prf, expectedSpace, expectedDigest, locationURL, expectedSize)
			bri, expectedReplicaCaveats := buildReplicateInvocation(
				t, lcd, expectedDigest, expectedSize, replicas,
			)

			// Condition: If existing allocation, store an existing allocation
			// coverage when an allocation has been made but not transfered.
			if tc.hasExistingAllocation {
				require.NoError(t, svc.Blobs().Allocations().Put(ctx, allocation.Allocation{
					Space: expectedSpace,
					Blob: allocation.Blob{
						Digest: expectedDigest,
						Size:   expectedSize,
					},
					Expires: uint64(time.Now().Add(time.Hour).UTC().Unix()),
					Cause:   bri.Link(),
				}))
			}

			// Condition: If existing data, store it in the blob store
			// covers when an allocation and replica already exist, meaning no transfer required.
			// though we still expect a transfer receipt.
			if tc.hasExistingData {
				require.NoError(t, svc.Blobs().Store().Put(
					ctx, expectedDigest, expectedSize, bytes.NewReader(expectedData),
				))
			}

			// Build + execute the actual replica.Allocate invocation.
			// simulating an upload service sending the invocation to the storage node.
			rbi, expectedAllocateCaveats := buildAllocateInvocation(
				t, bri, lcd, expectedSpace, expectedDigest, expectedSize,
			)
			res, err := client.Execute([]invocation.Invocation{rbi}, conn)
			require.NoError(t, err)

			// The final assertion on the returned allocation size.
			// With an existing allocation or existing data, the new allocated
			// size is 0, otherwise it’s expectedSize.
			var wantSize uint64
			if !tc.hasExistingAllocation && !tc.hasExistingData {
				wantSize = expectedSize
			}
			// read the receipt for the blob allocate, asserting its size is expected value.
			alloc := mustReadAllocationReceipt(t, rbi, res)
			require.EqualValues(t, wantSize, alloc.Size)

			// Assert that the Site promise field exists and has the correct structure
			require.NotNil(t, alloc.Site)
			require.Equal(t, ".out.ok", alloc.Site.UcanAwait.Selector)

			// "Wait" for the transfer invocation to produce a receipt
			// simulating the upload-service getting a receipt from this storage node.
			transferOkMsg := mustWaitForTransferMsg(t, ctx, transferOkChan)
			// expect one invocation and one receipt
			require.Len(t, transferOkMsg.Invocations(), 1)
			require.Len(t, transferOkMsg.Receipts(), 1)

			// Full read + assertion on the transfer invocation and its ucan chain
			mustAssertTransferInvocation(
				t,
				transferOkMsg,
				expectedDigest,
				wantSize,
				expectedSpace,
				expectedLocationCaveats,
				expectedAllocateCaveats,
				expectedReplicaCaveats,
			)
		})
	}
}

// Sets up the pre-signed URLs + returns them for use in testing
func setupURLs(
	t *testing.T,
	serverAddr string,
	sourcePath, sinkPath, uploadServicePath string,
) (*url.URL, *url.URL, *FakePresigned) {
	makeURL := func(path string) *url.URL {
		return testutil.Must(
			url.Parse(fmt.Sprintf("http://127.0.0.1%s/%s", serverAddr, path)),
		)(t)
	}
	locationURL := makeURL(sourcePath)
	uploadServiceURL := makeURL(uploadServicePath)
	presignedURL := makeURL(sinkPath)
	fakeBlobPresigner := &FakePresigned{uploadURL: *presignedURL}
	return locationURL, uploadServiceURL, fakeBlobPresigner
}

// Creates + starts your main service with custom dependencies
func setupService(
	t *testing.T,
	fakeBlobPresigner presigner.RequestPresigner,
	uploadServiceURL *url.URL,
) (servicetypes.Service, *ucan.Server) {
	// Create test config with custom upload service URL
	cfg := app.Config{
		ID:               testutil.Alice,
		UploadServiceURL: uploadServiceURL,
		UploadServiceDID: testutil.Alice.DID(),
	}
	cfg = app.ApplyDefaults(cfg)

	// Create test app with all necessary modules
	var svc servicetypes.Service
	var svr *ucan.Server
	app := fxtest.New(t,
		// Supply the test configuration
		fx.Supply(cfg),
		// an in-memory datastore implementation
		datastores.MemoryModule,
		// HTTP server
		server.Module,
		// UCAN server
		ucan.Module,
		// service configurations
		config.Module,
		// service implementations
		services.ServiceModule,
		// provide replicator service ucan methods relevant to the test
		replicator.UCANModule,
		// Override the presigner by decorating the provider
		fx.Decorate(func(cfg app.Config) presigner.RequestPresigner {
			return fakeBlobPresigner
		}),
		// Extract services for testing
		fx.Populate(&svc, &svr),
	)
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})

	return svc, svr
}

// Builds the UCAN delegation proof needed for replicate + allocate
func buildDelegationProof(t *testing.T) delegation.Delegation {
	caps := []ucanto.Capability[ucanto.CaveatBuilder]{
		ucanto.NewCapability(replica.AllocateAbility, testutil.Alice.DID().String(), ucanto.CaveatBuilder(ok.Unit{})),
		ucanto.NewCapability(blob.AllocateAbility, testutil.Alice.DID().String(), ucanto.CaveatBuilder(ok.Unit{})),
		ucanto.NewCapability(blob.AcceptAbility, testutil.Alice.DID().String(), ucanto.CaveatBuilder(ok.Unit{})),
	}
	d := testutil.Must(
		delegation.Delegate(testutil.Alice, testutil.Service, caps),
	)(t)
	return d
}

// Builds the location claim
func buildLocationClaim(
	t *testing.T,
	prf delegation.Delegation,
	space did.DID,
	digest multihash.Multihash,
	locationURL *url.URL,
	size uint64,
) (delegation.Delegation, assert.LocationCaveats) {
	locCav := assert.LocationCaveats{
		Space:    space,
		Content:  types.FromHash(digest),
		Location: []url.URL{*locationURL},
		Range:    &assert.Range{Offset: 1, Length: &size},
	}
	lcd, err := assert.Location.Delegate(
		testutil.Alice,
		testutil.Alice.DID(),
		testutil.Alice.DID().String(),
		locCav,
		delegation.WithProof(delegation.FromDelegation(prf)),
	)
	require.NoError(t, err)
	return lcd, locCav
}

// Builds the replicate invocation + attaches location claim
func buildReplicateInvocation(
	t *testing.T,
	lcd delegation.Delegation,
	digest multihash.Multihash,
	size uint64,
	replicas uint,
) (invocation.Invocation, spaceblob.ReplicateCaveats) {
	expectedReplicaCaveats := spaceblob.ReplicateCaveats{
		Blob: types.Blob{
			Digest: digest,
			Size:   size,
		},
		Replicas: replicas,
		Site:     lcd.Root().Link(),
	}
	bri, err := spaceblob.Replicate.Invoke(
		testutil.Alice,
		testutil.Alice.DID(),
		testutil.Alice.DID().String(),
		expectedReplicaCaveats,
	)
	require.NoError(t, err)

	// attach location claim blocks
	for block, err := range lcd.Blocks() {
		require.NoError(t, err)
		require.NoError(t, bri.Attach(block))
	}
	return bri, expectedReplicaCaveats
}

// Builds the replica allocate invocation + attaches replicate blocks
func buildAllocateInvocation(
	t *testing.T,
	bri invocation.Invocation,
	lcd delegation.Delegation,
	space did.DID,
	digest multihash.Multihash,
	size uint64,
) (invocation.Invocation, replica.AllocateCaveats) {
	expectedAllocateCaveats := replica.AllocateCaveats{
		Space: space,
		Blob:  types.Blob{Digest: digest, Size: size},
		Site:  lcd.Root().Link(),
		Cause: bri.Root().Link(),
	}
	rbi, err := replica.Allocate.Invoke(
		testutil.Alice,
		testutil.Alice.DID(),
		testutil.Alice.DID().String(),
		expectedAllocateCaveats,
	)
	require.NoError(t, err)

	// attach replicate invocation blocks
	for block, err := range bri.Blocks() {
		require.NoError(t, err)
		require.NoError(t, rbi.Attach(block))
	}
	return rbi, expectedAllocateCaveats
}

// Unwrap and read the receipt that returns the replica.AllocateOk
func mustReadAllocationReceipt(
	t *testing.T,
	rbi invocation.Invocation,
	res client.ExecutionResponse,
) replica.AllocateOk {
	reader, err := receipt.NewReceiptReaderFromTypes[replica.AllocateOk, fdm.FailureModel](
		replica.AllocateOkType(), fdm.FailureType(), types.Converters...,
	)
	require.NoError(t, err)

	rcptLink, ok := res.Get(rbi.Link())
	require.True(t, ok)

	rcpt, err := reader.Read(rcptLink, res.Blocks())
	require.NoError(t, err)

	alloc, err := result.Unwrap(result.MapError(rcpt.Out(), failure.FromFailureModel))
	require.NoError(t, err)
	return alloc
}

// Wait for the transfer message from the test HTTP server
func mustWaitForTransferMsg(
	t *testing.T,
	ctx context.Context,
	ch <-chan message.AgentMessage,
) message.AgentMessage {
	select {
	case <-ctx.Done():
		t.Fatal("test did not produce transfer receipt in time: ", ctx.Err())
		return nil
	case transferOkMsg := <-ch:
		require.NotNil(t, transferOkMsg)
		return transferOkMsg
	}
}

// Reads the final “transfer invocation” and asserts its fields, and chain of invocations
func mustAssertTransferInvocation(
	t *testing.T,
	transferOkMsg message.AgentMessage,
	expectedDigest multihash.Multihash,
	expectedSize uint64,
	expectedSpace did.DID,
	expectedLocationCav assert.LocationCaveats,
	expectedAllocateCav replica.AllocateCaveats,
	expectedReplicaCav spaceblob.ReplicateCaveats,
) {
	// sanity check
	require.NotNil(t, transferOkMsg)

	// create a reader for the transfer invocation chain.
	transferInvocationCid := testutil.Must(
		cid.Parse(transferOkMsg.Invocations()[0].String()),
	)(t)
	reader := testutil.Must(
		blockstore.NewBlockReader(blockstore.WithBlocksIterator(transferOkMsg.Blocks())),
	)(t)

	// get the transfer caveats and assert they match expected values
	transferCav := mustGetInvocationCaveats[replica.TransferCaveats](
		t, reader, cidlink.Link{Cid: transferInvocationCid},
		replica.TransferCaveatsReader.Read,
	)
	require.EqualValues(t, expectedSize, transferCav.Blob.Size)
	require.Equal(t, expectedDigest, transferCav.Blob.Digest)
	require.Equal(t, expectedSpace, transferCav.Space)

	// extract the location claim from the transfer invocation
	locationCav := mustGetInvocationCaveats[assert.LocationCaveats](
		t, reader, transferCav.Site, assert.LocationCaveatsReader.Read,
	)
	require.Equal(t, expectedLocationCav, locationCav)

	// verify cause -> points back to replica allocate
	replicaAllocateCav := mustGetInvocationCaveats[replica.AllocateCaveats](
		t, reader, transferCav.Cause, replica.AllocateCaveatsReader.Read,
	)
	require.Equal(t, expectedAllocateCav, replicaAllocateCav)

	// verify replica allocate cause is blob replicate
	blobReplicateCav := mustGetInvocationCaveats[spaceblob.ReplicateCaveats](
		t, reader, replicaAllocateCav.Cause, spaceblob.ReplicateCaveatsReader.Read,
	)
	require.Equal(t, expectedReplicaCav, blobReplicateCav)

	// read the transfer receipt
	transferReceiptCid := testutil.Must(
		cid.Parse(transferOkMsg.Receipts()[0].String()),
	)(t)
	transferReceiptReader := testutil.Must(
		receipt.NewReceiptReaderFromTypes[replica.TransferOk, fdm.FailureModel](
			replica.TransferOkType(), fdm.FailureType(), types.Converters...,
		),
	)(t)
	transferReceipt := testutil.Must(
		transferReceiptReader.Read(cidlink.Link{Cid: transferReceiptCid}, reader.Iterator()),
	)(t)
	transferOk := testutil.Must(
		result.Unwrap(result.MapError(transferReceipt.Out(), failure.FromFailureModel)),
	)(t)

	// PDP isn't enabled in this test setup, so no PDP proof expected.
	require.Nil(t, transferOk.PDP)

	// read the receipt of the transfer invocation asserting the location caveats of Site contain expected values.
	locationCavRct := mustGetInvocationCaveats[assert.LocationCaveats](t, reader, transferOk.Site, assert.LocationCaveatsReader.Read)
	require.Equal(t, expectedSpace, locationCavRct.Space)
	require.Equal(t, expectedDigest, locationCavRct.Content.Hash())
	require.Len(t, locationCavRct.Location, 1)
	require.Equal(t, fmt.Sprintf("/blob/z%s", expectedDigest.B58String()), locationCavRct.Location[0].Path)
}

func mustGetInvocationCaveats[T ipld.Builder](t *testing.T, reader blockstore.BlockReader, inv ucanto.Link, invReader func(any) (T, failure.Failure)) T {
	view := testutil.Must(invocation.NewInvocationView(inv, reader))(t)
	invc := testutil.Must(invReader(view.Capabilities()[0].Nb()))(t)
	return invc
}

// startTestHTTPServer starts a simple HTTP server with configurable endpoints.
func startTestHTTPServer(
	ctx context.Context,
	t *testing.T,
	digest multihash.Multihash,
	serveData []byte,
	svc servicetypes.Service,
	addr, sourcePath, sinkPath, uploadServicePath string,
) (*http.Server, <-chan message.AgentMessage) {
	agentCh := make(chan message.AgentMessage, 1)
	mux := http.NewServeMux()

	// Endpoint to serve data.
	mux.HandleFunc(fmt.Sprintf("/%s", sourcePath), func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(serveData)
	})
	// Endpoint to store data on the replica.
	mux.HandleFunc(fmt.Sprintf("/%s", sinkPath), func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, svc.Blobs().Store().Put(ctx, digest, uint64(len(serveData)), bytes.NewReader(serveData)))
		_, _ = w.Write(serveData)
	})
	// Endpoint to simulate the upload service.
	mux.HandleFunc(fmt.Sprintf("/%s", uploadServicePath), func(w http.ResponseWriter, r *http.Request) {
		roots, blocks, err := car.Decode(r.Body)
		require.NoError(t, err)
		bstore, err := blockstore.NewBlockReader(blockstore.WithBlocksIterator(blocks))
		require.NoError(t, err)
		agentMessage, err := message.NewMessage(roots, bstore)
		require.NoError(t, err)
		agentCh <- agentMessage
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	var listenErr error
	go func() {
		if err := server.ListenAndServe(); err != nil {
			listenErr = err
		}
	}()
	time.Sleep(500 * time.Millisecond)
	require.NoError(t, listenErr)
	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})
	return server, agentCh
}

// FakePresigned is a stub for upload URL presigning.
// TODO turn this into a mock
type FakePresigned struct {
	uploadURL url.URL
}

func (f *FakePresigned) SignUploadURL(ctx context.Context, digest multihash.Multihash, size, ttl uint64) (url.URL, http.Header, error) {
	return f.uploadURL, nil, nil
}

func (f *FakePresigned) VerifyUploadURL(ctx context.Context, url url.URL, headers http.Header) (url.URL, http.Header, error) {
	// TODO: implement when needed.
	panic("implement me")
}
