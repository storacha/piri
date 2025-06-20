package services_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/result"
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
	"github.com/storacha/piri/pkg/server"
	"github.com/storacha/piri/pkg/services"
	blobsvc "github.com/storacha/piri/pkg/services/blob"
	serviceconfig "github.com/storacha/piri/pkg/services/config"
	servicetypes "github.com/storacha/piri/pkg/services/types"
	"github.com/storacha/piri/pkg/services/ucan"
)

// testConfig creates a minimal storage.Config for testing
func testConfig() app.Config {
	cfg := app.Config{
		ID: testutil.Alice,
	}
	// Apply defaults to ensure all required fields are set
	return app.ApplyDefaults(cfg)
}

func TestServer(t *testing.T) {
	// Create test app with all necessary modules
	var svc servicetypes.Service
	var svr *ucan.Server
	app := fxtest.New(t,
		// Supply the test configuration
		fx.Supply(testConfig()),
		// an in-memory datastore implementation
		datastores.MemoryModule,
		// HTTP server
		server.Module,
		// UCAN server
		ucan.Module,
		// service configurations
		serviceconfig.Module,
		// service implementations
		services.ServiceModule,
		// provide blob service ucan methods relevant to the test
		blobsvc.UCANModule,
		// Extract services for testing
		fx.Populate(&svc, &svr),
	)
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})

	// Create UCAN connection for testing
	conn := testutil.Must(client.NewConnection(testutil.Service, svr))(t)

	// Create delegation proof
	prf := delegation.FromDelegation(
		testutil.Must(
			delegation.Delegate(
				testutil.Alice,
				testutil.Service,
				[]ucanto.Capability[ucanto.CaveatBuilder]{
					ucanto.NewCapability(
						blob.AllocateAbility,
						testutil.Alice.DID().String(),
						ucanto.CaveatBuilder(ok.Unit{}),
					),
					ucanto.NewCapability(
						blob.AcceptAbility,
						testutil.Alice.DID().String(),
						ucanto.CaveatBuilder(ok.Unit{}),
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

		resp, err := client.Execute([]invocation.Invocation{inv}, conn)
		require.NoError(t, err)

		// get the receipt link for the invocation from the response
		rcptlnk, ok := resp.Get(inv.Link())
		require.True(t, ok, "missing receipt for invocation: %s", inv.Link())

		reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AllocateOk, fdm.FailureModel](blob.AllocateOkType(), fdm.FailureType(), types.Converters...))(t)
		rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)

		result.MatchResultR0(rcpt.Out(), func(ok blob.AllocateOk) {
			fmt.Printf("%+v\n", ok)
			require.Equal(t, size, uint64(ok.Size))

			allocs, err := svc.Blobs().Allocations().List(context.Background(), digest)
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

			resp, err := client.Execute([]invocation.Invocation{inv}, conn)
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
		err := svc.Blobs().Store().Put(context.Background(), digest, size, bytes.NewReader(data))
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

			resp, err := client.Execute([]invocation.Invocation{inv}, conn)
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
		err := svc.Blobs().Store().Put(context.Background(), digest, size, bytes.NewReader(data))
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

		_, err = client.Execute([]invocation.Invocation{allocInv}, conn)
		require.NoError(t, err)

		// simulate a blob upload
		err = svc.Blobs().Store().Put(context.Background(), digest, size, bytes.NewReader(data))
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

		resp, err := client.Execute([]invocation.Invocation{acceptInv}, conn)
		require.NoError(t, err)

		// get the receipt link for the invocation from the response
		rcptlnk, ok := resp.Get(acceptInv.Link())
		require.True(t, ok, "missing receipt for invocation: %s", acceptInv.Link())

		reader := testutil.Must(receipt.NewReceiptReaderFromTypes[blob.AcceptOk, fdm.FailureModel](blob.AcceptOkType(), fdm.FailureType(), types.Converters...))(t)
		rcpt := testutil.Must(reader.Read(rcptlnk, resp.Blocks()))(t)

		result.MatchResultR0(rcpt.Out(), func(ok blob.AcceptOk) {
			fmt.Printf("%+v\n", ok)

			claim, err := svc.Claims().Store().Get(context.Background(), ok.Site)
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
