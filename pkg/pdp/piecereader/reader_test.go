package piecereader_test

import (
	"bytes"
	"io"
	"net/url"
	"testing"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/internal/mocks"
	"github.com/storacha/piri/pkg/pdp/piecereader"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReadPiece(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mocks.NewMockAPI(ctrl)
	pr := piecereader.New(clientMock, testutil.Must(url.Parse("https://example.com"))(t))

	expectedSize := int64(1024)
	expectedData := testutil.RandomBytes(t, 32)

	piece := testutil.RandomPiece(t, 1024)
	pieceCid := piece.Link().(cidlink.Link).Cid

	clientMock.EXPECT().
		ReadPiece(ctx, pieceCid).
		Return(&types.PieceReader{
			Size: expectedSize,
			Data: io.NopCloser(bytes.NewReader(expectedData)),
		})

	ret, err := pr.ReadPiece(ctx, pieceCid)
	require.NoError(t, err)
	require.Equal(t, expectedSize, ret.Size)
	require.Equal(t, expectedData, testutil.Must(io.ReadAll(ret.Data))(t))
}
