package egresstracking

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/capabilities/space/content"
	"github.com/storacha/go-libstoracha/capabilities/space/egress"
	captypes "github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/receipt/ran"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	fdm "github.com/storacha/go-ucanto/core/result/failure/datamodel"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/store/retrievaljournal"
	"github.com/stretchr/testify/require"
)

func TestAddReceipt(t *testing.T) {
	thisNode := testutil.RandomSigner(t)

	// Create mock UCAN server
	mockServer := NewMockEgressTrackerServer(t)

	// Create a test batch endpoint
	batchEndpoint, err := url.Parse("http://storage.node/receipts/{cid}")
	require.NoError(t, err)

	// Create egress tracker proof
	eTrackerDlg, err := delegation.Delegate(
		testutil.Service,
		thisNode,
		[]ucan.Capability[ucan.NoCaveats]{
			ucan.NewCapability(
				egress.TrackAbility,
				testutil.Service.DID().String(),
				ucan.NoCaveats{},
			),
		},
		delegation.WithNoExpiration(),
	)
	require.NoError(t, err)

	// Setup egress tracker connection
	eTrackerURL, err := url.Parse(mockServer.URL())
	require.NoError(t, err)
	ch := ucanhttp.NewChannel(eTrackerURL)
	eTrackerConn, err := client.NewConnection(testutil.Service, ch)
	require.NoError(t, err)

	t.Run("enqueues an egress track task on full batches", func(t *testing.T) {
		// Create a test store
		tempDir := t.TempDir()
		store, err := retrievaljournal.NewFSJournal(tempDir, 100) // 100 bytes batch size
		require.NoError(t, err)
		queue := NewMockEgressTrackingQueue(t)

		// Create service
		service, err := New(
			thisNode,
			eTrackerConn,
			delegation.Proofs{delegation.FromDelegation(eTrackerDlg)},
			batchEndpoint,
			store,
			queue,
			0, // cleanup disabled for tests
		)
		require.NoError(t, err)

		// Create a test receipt
		rcpt := createTestReceipt(t, testutil.Alice, thisNode)

		// Test adding a receipt. Max batch size is 100 bytes, so this should trigger a batch rotation.
		err = service.AddReceipt(t.Context(), rcpt)
		require.NoError(t, err)

		// Verify the batch was sent to the egress tracker
		require.Len(t, mockServer.Invocations(), 1, "expected one egress track invocation")
		require.Len(t, mockServer.BatchCIDs(), 1, "expected one batch CID")

		mockServer.Reset()
	})

	t.Run("concurrent addition", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := retrievaljournal.NewFSJournal(tempDir, 1024)
		require.NoError(t, err)
		queue := NewMockEgressTrackingQueue(t)

		// Create service
		service, err := New(
			thisNode,
			eTrackerConn,
			delegation.Proofs{delegation.FromDelegation(eTrackerDlg)},
			batchEndpoint,
			store,
			queue,
			0, // cleanup disabled for tests
		)
		require.NoError(t, err)

		var wg sync.WaitGroup
		numReceipts := 10

		// Create multiple goroutines to add receipts concurrently
		for range numReceipts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rcpt := createTestReceipt(t, testutil.Alice, thisNode)
				err := service.AddReceipt(t.Context(), rcpt)
				require.NoError(t, err)
			}()
		}

		wg.Wait()

		// Verify the egress tracker was invoked
		require.True(t, len(mockServer.Invocations()) > 0, "no egress track invocations sent")
	})
}

func createTestReceipt(t *testing.T, client ucan.Signer, node ucan.Signer) receipt.Receipt[content.RetrieveOk, fdm.FailureModel] {
	space := testutil.RandomDID(t)
	inv, err := content.Retrieve.Invoke(
		client,
		node,
		space.String(),
		content.RetrieveCaveats{
			Blob: content.BlobDigest{
				Digest: testutil.RandomMultihash(t),
			},
			Range: content.Range{
				Start: 1024,
				End:   2048,
			},
		},
	)
	require.NoError(t, err)

	ran := ran.FromInvocation(inv)
	ok := result.Ok[content.RetrieveOk, failure.IPLDBuilderFailure](content.RetrieveOk{})
	rcpt, err := receipt.Issue(
		node,
		ok,
		ran,
	)
	require.NoError(t, err)

	retrieveRcpt, err := receipt.Rebind[content.RetrieveOk, fdm.FailureModel](rcpt, content.RetrieveOkType(), fdm.FailureType(), captypes.Converters...)
	require.NoError(t, err)

	return retrieveRcpt
}

type MockEgressTrackingQueue struct {
	t  *testing.T
	fn func(ctx context.Context, batchCID cid.Cid) error
}

func NewMockEgressTrackingQueue(t *testing.T) *MockEgressTrackingQueue {
	return &MockEgressTrackingQueue{t: t}
}

func (m *MockEgressTrackingQueue) Register(fn func(ctx context.Context, batchCID cid.Cid) error) error {
	m.fn = fn
	return nil
}

func (m *MockEgressTrackingQueue) Enqueue(ctx context.Context, batchCID cid.Cid) error {
	if m.fn == nil {
		m.t.Fatal("no enqueue function registered")
	}
	return m.fn(ctx, batchCID)
}

// MockEgressTrackerServer is a mock UCAN server that handles egress track invocations
type MockEgressTrackerServer struct {
	server *httptest.Server
	mu     sync.Mutex
	t      *testing.T

	// Track invocations
	invocations []invocation.Invocation
	batchCIDs   []cid.Cid
}

// NewMockEgressTrackerServer creates a new mock UCAN server for testing
func NewMockEgressTrackerServer(t *testing.T) *MockEgressTrackerServer {
	m := &MockEgressTrackerServer{
		t:           t,
		invocations: make([]invocation.Invocation, 0),
		batchCIDs:   make([]cid.Cid, 0),
	}

	ucansrv, err := ucanserver.NewServer(testutil.Service, m.egressTrack())
	if err != nil {
		t.Fatalf("failed to create UCAN server: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.ucanHandler(ucansrv))

	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)

	return m
}

// URL returns the base URL of the mock server
func (m *MockEgressTrackerServer) URL() string {
	return m.server.URL
}

// Reset clears all recorded invocations
func (m *MockEgressTrackerServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations = m.invocations[:0]
	m.batchCIDs = m.batchCIDs[:0]
}

// handleTrack handles the space/egress/track UCAN invocation
func (m *MockEgressTrackerServer) egressTrack() ucanserver.Option {
	return ucanserver.WithServiceMethod(
		egress.TrackAbility,
		ucanserver.Provide(
			egress.Track,
			func(
				ctx context.Context,
				cap ucan.Capability[egress.TrackCaveats],
				inv invocation.Invocation,
				iCtx ucanserver.InvocationContext,
			) (result.Result[egress.TrackOk, failure.IPLDBuilderFailure], fx.Effects, error) {
				// Record the invocation and batch CID
				m.mu.Lock()
				defer m.mu.Unlock()

				m.invocations = append(m.invocations, inv)
				m.batchCIDs = append(m.batchCIDs, cap.Nb().Receipts.(cidlink.Link).Cid)

				return result.Ok[egress.TrackOk, failure.IPLDBuilderFailure](egress.TrackOk{}), nil, nil
			},
		),
	)
}

func (m *MockEgressTrackerServer) ucanHandler(srv ucanserver.ServerView[ucanserver.Service]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := srv.Request(r.Context(), ucanhttp.NewRequest(r.Body, r.Header))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for key, vals := range res.Headers() {
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}

		// content type is empty as it will have been set by ucanto transport codec
		w.WriteHeader(res.Status())
		respBody, err := io.ReadAll(res.Body())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(respBody)
	}
}

// Invocations returns all recorded invocations
func (m *MockEgressTrackerServer) Invocations() []invocation.Invocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]invocation.Invocation{}, m.invocations...)
}

// BatchCIDs returns all recorded batch CIDs
func (m *MockEgressTrackerServer) BatchCIDs() []cid.Cid {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]cid.Cid{}, m.batchCIDs...)
}
