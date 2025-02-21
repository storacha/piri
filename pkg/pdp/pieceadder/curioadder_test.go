package pieceadder_test

import (
	"context"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/storage/internal/mocks"
	"github.com/storacha/storage/pkg/internal/testutil"
	"github.com/storacha/storage/pkg/pdp/curio"
	"github.com/storacha/storage/pkg/pdp/pieceadder"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddPiece(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockPDPClient(ctrl)

	pa := pieceadder.NewCurioAdder(clientMock)

	expectedMh := testutil.RandomMultihash(t)
	expectedDigest := testutil.Must(multihash.Decode(expectedMh))(t)
	expectedSize := uint64(1028)
	expectedURL := testutil.Must(url.Parse("http://example.com"))(t)

	clientMock.EXPECT().AddPiece(ctx, curio.AddPiece{
		Check: curio.PieceHash{
			Name: expectedDigest.Name,
			Size: int64(expectedSize),
			Hash: hex.EncodeToString(expectedDigest.Digest),
		},
	}).Return(&curio.UploadRef{URL: expectedURL.String()}, nil)

	actualURL, err := pa.AddPiece(ctx, expectedMh, expectedSize)
	require.NoError(t, err)
	require.Equal(t, expectedURL, actualURL)
}
