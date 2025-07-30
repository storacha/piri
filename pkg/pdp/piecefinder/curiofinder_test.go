package piecefinder_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/piri/internal/mocks"
	"github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/pdp/curio"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFindPiece(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)
	pa := piecefinder.NewCurioFinder(clientMock)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)
	expectedDigest := testutil.Must(multihash.Decode(expectedMh))(t)
	expectedPiece := testutil.CreatePiece(t, 1024)
	clientMock.EXPECT().
		FindPiece(ctx, curio.PieceHash{
			Hash: hex.EncodeToString(expectedDigest.Digest),
			Name: expectedDigest.Name,
			Size: int64(expectedSize),
		}).
		Return(curio.FoundPiece{PieceCID: expectedPiece.V1Link().String()}, nil)

	_, err := pa.FindPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
}

func TestFindPiece_RetryThenSuccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)
	maxAttempts := 10
	retryDelay := 50 * time.Millisecond
	finder := piecefinder.NewCurioFinder(clientMock, piecefinder.WithMaxAttempts(maxAttempts), piecefinder.WithRetryDelay(retryDelay))

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)
	expectedPiece := testutil.CreatePiece(t, 1024)

	// First 2 calls return a 404-like error, third call succeeds
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(curio.FoundPiece{}, curio.ErrFailedResponse{StatusCode: http.StatusNotFound}).
		Times(2)

	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(curio.FoundPiece{PieceCID: expectedPiece.V1Link().String()}, nil).
		Times(1)

	res, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
	require.Equal(t, expectedPiece.V1Link().String(), res.V1Link().String())
}

func TestFindPiece_ExceedMaxRetries(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)
	maxAttempts := 10
	retryDelay := 50 * time.Millisecond
	finder := piecefinder.NewCurioFinder(clientMock, piecefinder.WithMaxAttempts(maxAttempts), piecefinder.WithRetryDelay(retryDelay))

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)

	// Return 404 each time to exceed maxAttempts
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(curio.FoundPiece{}, curio.ErrFailedResponse{StatusCode: http.StatusNotFound}).
		Times(maxAttempts)

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
}

func TestFindPiece_UnexpectedError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)
	finder := piecefinder.NewCurioFinder(clientMock)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)

	// First 2 calls return a 404-like error, third call succeeds
	mockErr := fmt.Errorf("unexpected server error")
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(curio.FoundPiece{}, mockErr).
		Times(1)

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
	require.Equal(t, mockErr, err)
}

func TestFindPiece_ContextCanceled(t *testing.T) {
	// Use a short retry delay to keep the test quick
	ctx, cancel := context.WithCancel(t.Context())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)
	finder := piecefinder.NewCurioFinder(clientMock)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)
	expectedDigest := testutil.Must(multihash.Decode(expectedMh))(t)

	// First call returns a 404; we cancel the context before we get to the second retry
	clientMock.EXPECT().
		FindPiece(ctx, curio.PieceHash{
			Hash: hex.EncodeToString(expectedDigest.Digest),
			Name: expectedDigest.Name,
			Size: int64(expectedSize),
		}).
		Return(curio.FoundPiece{}, curio.ErrFailedResponse{StatusCode: http.StatusNotFound}).
		Times(1)

	// Cancel the context here so the second attempt never really happens
	cancel()

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
