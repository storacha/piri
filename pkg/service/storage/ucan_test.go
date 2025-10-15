package storage

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
	blob2 "github.com/storacha/go-libstoracha/capabilities/space/blob"
	"github.com/storacha/go-libstoracha/capabilities/types"
	ucancap "github.com/storacha/go-libstoracha/capabilities/ucan"
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
	"github.com/storacha/go-ucanto/ucan"
	testutil2 "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/stretchr/testify/require"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

func TestServer(t *testing.T) {
	ctx := t.Context()
	svc, err := New(WithIdentity(testutil.Alice), WithLogLevel("*", "warn"))
	require.NoError(t, err)
	err = svc.Startup(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		svc.Close(ctx)
	})

	srv, err := NewUCANServer(svc)
	require.NoError(t, err)

	conn := testutil.Must(client.NewConnection(testutil.Service, srv))(t)

	prf := delegation.FromDelegation(
		testutil.Must(
			delegation.Delegate(
				testutil.Alice,
				testutil.Service,
				[]ucan.Capability[ucan.CaveatBuilder]{
					ucan.NewCapability(
						blob.AllocateAbility,
						testutil.Alice.DID().String(),
						ucan.CaveatBuilder(ok.Unit{}),
					),
					ucan.NewCapability(
						blob.AcceptAbility,
						testutil.Alice.DID().String(),
						ucan.CaveatBuilder(ok.Unit{}),
					),
				},
			),
		)(t),
	)

	t.Run("blob/allocate", func(t *testing.T) {
		space := testutil.RandomDID(t)
		digest := testutil.RandomMultihash(t)
		size := uint64(rand.IntN(32) + 1)
		cause := testutil.RandomCID(t)

		nb := blob.AllocateCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Cause: cause,
		}
		cap := blob.Allocate.New(testutil.Alice.DID().String(), nb)
		inv, err := invocation.Invoke(testutil.Service, testutil.Alice, cap, delegation.WithProof(prf))
		require.NoError(t, err)

		resp, err := client.Execute(ctx, []invocation.Invocation{inv}, conn)
		require.NoError(t, err)

		// get the receipt link for the invocation from the response
		rcptlnk, ok := resp.Get(inv.Link())
		require.True(t, ok, "missing receipt for invocation: %s", inv.Link())

		reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AllocateOk, fdm.FailureModel](blob.AllocateOkType(), fdm.FailureType(), types.Converters...))(t)
		rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)

		result.MatchResultR0(rcpt.Out(), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, size, uint64(ok.Size))

			allocs, err := svc.Blobs().Allocations().List(t.Context(), digest)
			require.NoError(t, err)

			require.Len(t, allocs, 1)
			require.Equal(t, digest, allocs[0].Blob.Digest)
			require.Equal(t, size, allocs[0].Blob.Size)
			require.Equal(t, space, allocs[0].Space)
			require.Equal(t, inv.Link(), allocs[0].Cause)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})
	})

	t.Run("repeat blob/allocate for same blob", func(t *testing.T) {
		space := testutil.RandomDID(t)
		size := uint64(rand.IntN(32) + 1)
		data := testutil.RandomBytes(t, int(size))
		digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)
		cause := testutil.RandomCID(t)

		nb := blob.AllocateCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Cause: cause,
		}
		cap := blob.Allocate.New(testutil.Alice.DID().String(), nb)

		invokeBlobAllocate := func() result.Result[blob.AllocateOk, fdm.FailureModel] {
			inv, err := invocation.Invoke(testutil.Service, testutil.Alice, cap, delegation.WithProof(prf))
			require.NoError(t, err)

			resp, err := client.Execute(ctx, []invocation.Invocation{inv}, conn)
			require.NoError(t, err)

			rcptlnk, ok := resp.Get(inv.Link())
			require.True(t, ok, "missing receipt for invocation: %s", inv.Link())

			reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AllocateOk, fdm.FailureModel](blob.AllocateOkType(), fdm.FailureType(), types.Converters...))(t)
			rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)
			return rcpt.Out()
		}

		result.MatchResultR0(invokeBlobAllocate(), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, size, uint64(ok.Size))
			require.NotNil(t, ok.Address)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})

		// now again without upload
		result.MatchResultR0(invokeBlobAllocate(), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, uint64(0), ok.Size)
			require.NotNil(t, ok.Address)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})

		// simulate a blob upload
		err = svc.Blobs().Store().Put(t.Context(), digest, size, bytes.NewReader(data))
		require.NoError(t, err)

		// now again after upload
		result.MatchResultR0(invokeBlobAllocate(), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, uint64(0), ok.Size)
			require.Nil(t, ok.Address)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})
	})

	t.Run("repeat blob/allocate for same blob in different space", func(t *testing.T) {
		space0 := testutil.RandomDID(t)
		space1 := testutil.RandomDID(t)
		size := uint64(rand.IntN(32) + 1)
		data := testutil.RandomBytes(t, int(size))
		digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)
		cause := testutil.RandomCID(t)

		invokeBlobAllocate := func(space did.DID) result.Result[blob.AllocateOk, fdm.FailureModel] {
			nb := blob.AllocateCaveats{
				Space: space,
				Blob: types.Blob{
					Digest: digest,
					Size:   size,
				},
				Cause: cause,
			}
			cap := blob.Allocate.New(testutil.Alice.DID().String(), nb)

			inv, err := invocation.Invoke(testutil.Service, testutil.Alice, cap, delegation.WithProof(prf))
			require.NoError(t, err)

			resp, err := client.Execute(ctx, []invocation.Invocation{inv}, conn)
			require.NoError(t, err)

			rcptlnk, ok := resp.Get(inv.Link())
			require.True(t, ok, "missing receipt for invocation: %s", inv.Link())

			reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AllocateOk, fdm.FailureModel](blob.AllocateOkType(), fdm.FailureType(), types.Converters...))(t)
			rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)
			return rcpt.Out()
		}

		result.MatchResultR0(invokeBlobAllocate(space0), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, size, uint64(ok.Size))
			require.NotNil(t, ok.Address)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})

		// simulate a blob upload
		err = svc.Blobs().Store().Put(t.Context(), digest, size, bytes.NewReader(data))
		require.NoError(t, err)

		// now again after upload, but in different space
		result.MatchResultR0(invokeBlobAllocate(space1), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, size, uint64(ok.Size))
			require.Nil(t, ok.Address)
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})
	})

	t.Run("blob/accept", func(t *testing.T) {
		space := testutil.RandomDID(t)
		size := uint64(rand.IntN(32) + 1)
		data := testutil.RandomBytes(t, int(size))
		digest := testutil.Must(multihash.Sum(data, multihash.SHA2_256, -1))(t)
		cause := testutil.RandomCID(t)

		allocNb := blob.AllocateCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Cause: cause,
		}
		allocCap := blob.Allocate.New(testutil.Alice.DID().String(), allocNb)
		allocInv, err := invocation.Invoke(testutil.Service, testutil.Alice, allocCap, delegation.WithProof(prf))
		require.NoError(t, err)

		_, err = client.Execute(ctx, []invocation.Invocation{allocInv}, conn)
		require.NoError(t, err)

		// simulate a blob upload
		err = svc.Blobs().Store().Put(t.Context(), digest, size, bytes.NewReader(data))
		require.NoError(t, err)
		// get the expected download URL
		loc, err := svc.Blobs().Access().GetDownloadURL(digest)
		require.NoError(t, err)

		// eventually service will invoke blob/accept
		acceptNb := blob.AcceptCaveats{
			Space: space,
			Blob: types.Blob{
				Digest: digest,
				Size:   size,
			},
			Put: blob.Promise{
				UcanAwait: blob.Await{
					Selector: ".out.ok",
					Link:     testutil.RandomCID(t),
				},
			},
		}
		// fmt.Println(printer.Sprint(testutil.Must(acceptNb.ToIPLD())(t)))
		acceptCap := blob.Accept.New(testutil.Alice.DID().String(), acceptNb)
		acceptInv, err := invocation.Invoke(testutil.Service, testutil.Alice, acceptCap, delegation.WithProof(prf))
		require.NoError(t, err)

		resp, err := client.Execute(ctx, []invocation.Invocation{acceptInv}, conn)
		require.NoError(t, err)

		// get the receipt link for the invocation from the response
		rcptlnk, ok := resp.Get(acceptInv.Link())
		require.True(t, ok, "missing receipt for invocation: %s", acceptInv.Link())

		reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AcceptOk, fdm.FailureModel](blob.AcceptOkType(), fdm.FailureType(), types.Converters...))(t)
		rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)

		result.MatchResultR0(rcpt.Out(), func(ok blob.AcceptOk) {
			fmt.Printf("%+v\n", ok)

			claim, err := svc.Claims().Store().Get(t.Context(), ok.Site)
			require.NoError(t, err)

			require.Equal(t, testutil.Alice.DID(), claim.Issuer())
			require.Equal(t, space, claim.Audience().DID())
			require.Equal(t, assert.LocationAbility, claim.Capabilities()[0].Can())
			require.Equal(t, testutil.Alice.DID().String(), claim.Capabilities()[0].With())

			nb, err := assert.LocationCaveatsReader.Read(claim.Capabilities()[0].Nb())
			require.NoError(t, err)

			require.Equal(t, space, nb.Space)
			require.Equal(t, digest, nb.Content.Hash())
			require.Equal(t, loc.String(), nb.Location[0].String())

			// TODO: assert IPNI advert published
		}, func(f fdm.FailureModel) {
			fmt.Println(f.Message)
			fmt.Println(*f.Stack)
			require.Nil(t, f)
		})

		require.NotEmpty(t, rcpt.Fx().Fork())
		effect := rcpt.Fx().Fork()[0]
		claim, ok := effect.Invocation()
		require.True(t, ok)
		require.Equal(t, assert.LocationAbility, claim.Capabilities()[0].Can())
	})
}

// TestReplicaAllocateTransfer validates the full replica allocation flow in the UCAN server,
// ensuring that invocations are correctly constructed and executed, and that the simulated endpoints
// interact as expected. A lightweight HTTP server (on port picked by OS) is used to simulate external endpoints:
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
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)

			// Common setup: random DID, random data, etc.
			expectedSpace := testutil.RandomDID(t)
			expectedSize := uint64(rand.IntN(32) + 1)
			expectedData := testutil.RandomBytes(t, int(expectedSize))
			expectedDigest := testutil.Must(
				multihash.Sum(expectedData, multihash.SHA2_256, -1),
			)(t)
			replicas := uint(1)
			port := testutil2.GetFreePort(t)
			serverAddr := fmt.Sprintf(":%d", port)
			sourcePath, sinkPath, uploadServicePath := "get", "put", "upload-service"

			// Spin up storage service, using injected values for testing.
			locationURL, uploadServiceURL, fakeBlobPresigner := setupURLs(t, serverAddr, sourcePath, sinkPath, uploadServicePath)
			svc := setupService(t, ctx, fakeBlobPresigner, uploadServiceURL)
			fakeServer, transferOkChan := startTestHTTPServer(
				ctx, t, expectedDigest, expectedData, svc,
				serverAddr, sourcePath, sinkPath, uploadServicePath,
			)
			t.Cleanup(func() {
				fakeServer.Close()
				svc.Close(ctx)
				cancel()
			})

			// Build UCAN server & connection
			srv, err := NewUCANServer(svc)
			require.NoError(t, err)
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
			// coverage when an allocation has been made but not transferred.
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
				require.NoError(t, svc.blobs.Store().Put(
					ctx, expectedDigest, expectedSize, bytes.NewReader(expectedData),
				))
			}

			// Build + execute the actual replica.Allocate invocation.
			// simulating an upload service sending the invocation to the storage node.
			rbi, expectedAllocateCaveats := buildAllocateInvocation(
				t, bri, lcd, expectedSpace, expectedDigest, expectedSize,
			)
			res, err := client.Execute(ctx, []invocation.Invocation{rbi}, conn)
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
			ucanConcludeMsg := mustWaitForTransferMsg(t, ctx, transferOkChan)
			// expect one invocation and 0 receipts
			require.Len(t, ucanConcludeMsg.Invocations(), 1)
			// receipt is attached to the invocation, not a reciept in the message
			require.Len(t, ucanConcludeMsg.Receipts(), 0)

			// Full read + assertion on the transfer invocation and its ucan chain
			mustAssertTransferInvocation(
				t,
				ucanConcludeMsg,
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

// Creates + starts your main service
func setupService(
	t *testing.T,
	ctx context.Context,
	fakeBlobPresigner *FakePresigned,
	uploadServiceURL *url.URL,
) *StorageService {
	svc, err := New(
		WithIdentity(testutil.Alice),
		WithLogLevel("*", "warn"),
		WithBlobsPresigner(fakeBlobPresigner),
		WithUploadServiceConfig(testutil.Alice, *uploadServiceURL),
	)
	require.NoError(t, err)
	require.NoError(t, svc.Startup(ctx))
	return svc
}

// Builds the UCAN delegation proof needed for replicate + allocate
func buildDelegationProof(t *testing.T) delegation.Delegation {
	caps := []ucan.Capability[ucan.CaveatBuilder]{
		ucan.NewCapability(replica.AllocateAbility, testutil.Alice.DID().String(), ucan.CaveatBuilder(ok.Unit{})),
		ucan.NewCapability(blob.AllocateAbility, testutil.Alice.DID().String(), ucan.CaveatBuilder(ok.Unit{})),
		ucan.NewCapability(blob.AcceptAbility, testutil.Alice.DID().String(), ucan.CaveatBuilder(ok.Unit{})),
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
) (invocation.Invocation, blob2.ReplicateCaveats) {
	expectedReplicaCaveats := blob2.ReplicateCaveats{
		Blob: types.Blob{
			Digest: digest,
			Size:   size,
		},
		Replicas: replicas,
		Site:     lcd.Root().Link(),
	}
	bri, err := blob2.Replicate.Invoke(
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
	case ucanConcludeMsg := <-ch:
		require.NotNil(t, ucanConcludeMsg)
		return ucanConcludeMsg
	}
}

func MustAssertTransferInvocationUcanConcludeReceipt(
	t *testing.T,
	ucanConcludeMsg message.AgentMessage,
	expectedDigest multihash.Multihash,
	expectedSize uint64,
	expectedSpace did.DID,
	expectedLocationCav assert.LocationCaveats,
	expectedAllocateCav replica.AllocateCaveats,
	expectedReplicaCav blob2.ReplicateCaveats,
) {
	// sanity check
	require.NotNil(t, ucanConcludeMsg)

	concludeInvocationCid := testutil.Must(
		cid.Parse(ucanConcludeMsg.Invocations()[0].String()),
	)(t)
	reader := testutil.Must(
		blockstore.NewBlockReader(blockstore.WithBlocksIterator(ucanConcludeMsg.Blocks())),
	)(t)

	concludeCav := mustGetInvocationCaveats[ucancap.ConcludeCaveats](
		t, reader, cidlink.Link{Cid: concludeInvocationCid},
		ucancap.ConcludeCaveatsReader.Read,
	)
	someotherreader, err := receipt.NewReceiptReaderFromTypes[replica.TransferOk, fdm.FailureModel](replica.TransferOkType(), fdm.FailureType(), types.Converters...)
	require.NoError(t, err)

	rcpt, err := someotherreader.Read(concludeCav.Receipt, ucanConcludeMsg.Blocks())
	require.NoError(t, err)

	// get the transfer caveats and assert they match expected values
	transferCav := mustGetInvocationCaveats[replica.TransferCaveats](
		t, reader, rcpt.Ran().Link(),
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
	blobReplicateCav := mustGetInvocationCaveats[blob2.ReplicateCaveats](
		t, reader, replicaAllocateCav.Cause, blob2.ReplicateCaveatsReader.Read,
	)
	require.Equal(t, expectedReplicaCav, blobReplicateCav)

	// read the transfer receipt
	transferReceiptCid := testutil.Must(
		cid.Parse(rcpt.Root().Link().String()),
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

// Reads the final “transfer invocation” and asserts its fields, and chain of invocations
func mustAssertTransferInvocation(
	t *testing.T,
	ucanConcludeMsg message.AgentMessage,
	expectedDigest multihash.Multihash,
	expectedSize uint64,
	expectedSpace did.DID,
	expectedLocationCav assert.LocationCaveats,
	expectedAllocateCav replica.AllocateCaveats,
	expectedReplicaCav blob2.ReplicateCaveats,
) {
	// sanity check
	require.NotNil(t, ucanConcludeMsg)

	concludeInvocationCid := testutil.Must(
		cid.Parse(ucanConcludeMsg.Invocations()[0].String()),
	)(t)
	reader := testutil.Must(
		blockstore.NewBlockReader(blockstore.WithBlocksIterator(ucanConcludeMsg.Blocks())),
	)(t)

	concludeCav := mustGetInvocationCaveats[ucancap.ConcludeCaveats](
		t, reader, cidlink.Link{Cid: concludeInvocationCid},
		ucancap.ConcludeCaveatsReader.Read,
	)
	someotherreader, err := receipt.NewReceiptReaderFromTypes[replica.TransferOk, fdm.FailureModel](replica.TransferOkType(), fdm.FailureType(), types.Converters...)
	require.NoError(t, err)

	rcpt, err := someotherreader.Read(concludeCav.Receipt, ucanConcludeMsg.Blocks())
	require.NoError(t, err)

	// get the transfer caveats and assert they match expected values
	transferCav := mustGetInvocationCaveats[replica.TransferCaveats](
		t, reader, rcpt.Ran().Link(),
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
	blobReplicateCav := mustGetInvocationCaveats[blob2.ReplicateCaveats](
		t, reader, replicaAllocateCav.Cause, blob2.ReplicateCaveatsReader.Read,
	)
	require.Equal(t, expectedReplicaCav, blobReplicateCav)

	// read the transfer receipt
	transferReceiptCid := testutil.Must(
		cid.Parse(rcpt.Root().Link().String()),
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

func mustGetInvocationCaveats[T ipld.Builder](t *testing.T, reader blockstore.BlockReader, inv ucan.Link, invReader func(any) (T, failure.Failure)) T {
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
	svc Service,
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
		agentMessage, err := message.NewMessage(roots[0], bstore)
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
