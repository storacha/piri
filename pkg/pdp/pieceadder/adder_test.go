package pieceadder_test

import (
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/storacha/piri/internal/mocks"
	"github.com/storacha/piri/pkg/pdp/pieceadder"

	"github.com/storacha/piri/pkg/pdp/types"
)

func TestAddPiece(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)

	pa := pieceadder.New(clientMock, testutil.Must(url.Parse("http://example.com"))(t))

	expectedMh := testutil.RandomMultihash(t)
	expectedDigest := testutil.Must(multihash.Decode(expectedMh))(t)
	expectedSize := uint64(1028)
	expectedUUID := uuid.New()
	expectedURL := testutil.Must(url.Parse("http://example.com"))(t).JoinPath("pdp", "piece", "upload", expectedUUID.String())

	clientMock.EXPECT().AllocatePiece(ctx, types.PieceAllocation{
		Piece: types.Piece{
			Name: expectedDigest.Name,
			Size: int64(expectedSize),
			Hash: hex.EncodeToString(expectedDigest.Digest),
		},
	}).Return(&types.AllocatedPiece{
		Allocated: true,
		Piece:     cid.Undef,
		UploadID:  expectedUUID,
	}, nil)

	actualURL, err := pa.AddPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
	require.Equal(t, expectedURL, actualURL)
}
