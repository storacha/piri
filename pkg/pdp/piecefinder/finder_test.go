package piecefinder_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/storacha/piri/internal/mocks"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/types"
)

func TestFindPiece(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	pa := piecefinder.New(clientMock, testutil.Must(url.Parse("https://example.com"))(t))

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)
	expectedDigest := testutil.Must(multihash.Decode(expectedMh))(t)
	expectedPiece := testutil.RandomPiece(t, 1024)
	clientMock.EXPECT().
		FindPiece(ctx, types.Piece{
			Hash: hex.EncodeToString(expectedDigest.Digest),
			Name: expectedDigest.Name,
			Size: int64(expectedSize),
		}).
		Return(mustPieceLinkToCid(t, expectedPiece), true, nil)

	_, err := pa.FindPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
}

func TestFindPiece_RetryThenSuccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	maxAttempts := 10
	retryDelay := 50 * time.Millisecond
	finder := piecefinder.New(
		clientMock,
		testutil.Must(url.Parse("https://example.com"))(t),
		piecefinder.WithMaxAttempts(maxAttempts),
		piecefinder.WithRetryDelay(retryDelay),
	)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)
	expectedPiece := testutil.RandomPiece(t, 1024)

	// First 2 calls return a 404-like error, third call succeeds
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(cid.Undef, false, nil).
		Times(2)

	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(mustPieceLinkToCid(t, expectedPiece), true, nil).
		Times(1)

	res, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
	require.Equal(t, expectedPiece.Link().String(), res.Link().String())
}

func TestFindPiece_ExceedMaxRetries(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	maxAttempts := 10
	retryDelay := 50 * time.Millisecond
	finder := piecefinder.New(
		clientMock,
		testutil.Must(url.Parse("https://example.com"))(t),
		piecefinder.WithMaxAttempts(maxAttempts),
		piecefinder.WithRetryDelay(retryDelay),
	)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)

	// Return 404 each time to exceed maxAttempts
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(cid.Undef, false, nil).
		Times(maxAttempts)

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
}

func TestFindPiece_UnexpectedError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	finder := piecefinder.New(
		clientMock,
		testutil.Must(url.Parse("https://example.com"))(t),
	)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)

	// First 2 calls return a 404-like error, third call succeeds
	mockErr := fmt.Errorf("unexpected server error")
	clientMock.EXPECT().FindPiece(ctx, gomock.Any()).
		Return(cid.Undef, false, mockErr).
		Times(1)

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
	require.Equal(t, mockErr, errors.Unwrap(err))
}

func TestFindPiece_ContextCanceled(t *testing.T) {
	// Use a short retry delay to keep the test quick
	ctx, cancel := context.WithCancel(t.Context())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	finder := piecefinder.New(
		clientMock,
		testutil.Must(url.Parse("https://example.com"))(t),
	)

	expectedSize := uint64(1024)
	expectedMh := testutil.RandomMultihash(t)

	// First call returns a 404; we cancel the context before we get to the second retry
	clientMock.EXPECT().
		FindPiece(ctx, gomock.Any()).
		Return(cid.Undef, false, nil).
		Times(1)

	// Cancel the context here so the second attempt never really happens
	cancel()

	_, err := finder.FindPiece(ctx, expectedMh, expectedSize)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestURLForPiece(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	expectedURL := testutil.Must(url.Parse("https://example.com"))(t)
	pa := piecefinder.New(clientMock, expectedURL)

	expectedPiece := testutil.RandomPiece(t, 1024)

	ref, err := pa.URLForPiece(ctx, expectedPiece)
	require.NoError(t, err)
	require.Equal(t, expectedURL.JoinPath("piece", expectedPiece.Link().String()).String(), ref.String())

}

func mustPieceLinkToCid(t testing.TB, pl piece.PieceLink) cid.Cid {
	out, err := cid.Decode(pl.Link().String())
	require.NoError(t, err)
	return out
}
