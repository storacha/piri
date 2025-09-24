package adapter_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"testing"

	cid "github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/pdp/piecereader"
	"github.com/storacha/piri/pkg/pdp/store/adapter"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/stretchr/testify/require"
)

func TestBlobGetterAdapter(t *testing.T) {
	t.Run("gets a piece from blob hash", func(t *testing.T) {
		data := testutil.RandomBytes(t, 128)
		digest := testutil.MultihashFromBytes(t, data)
		digestStr := digestutil.Format(digest)
		pieceLink := testutil.RandomPiece(t, int64(len(data)))
		pieceLinkStr := pieceLink.Link().String()

		finder := mockPieceFinder{map[string]piece.PieceLink{digestStr: pieceLink}}
		reader := mockPieceReader{map[string][]byte{pieceLinkStr: data}}
		sizer := mockSizer{map[string]uint64{digestStr: uint64(len(data))}}

		getter := adapter.NewBlobGetterAdapter(&finder, &reader, &sizer)
		obj, err := getter.Get(t.Context(), digest)
		require.NoError(t, err)

		require.Equal(t, int64(len(data)), obj.Size())
		require.Equal(t, data, testutil.Must(io.ReadAll(obj.Body()))(t))
	})

	t.Run("gets a byte range of a piece from blob hash", func(t *testing.T) {
		data := testutil.RandomBytes(t, 128)
		digest := testutil.MultihashFromBytes(t, data)
		digestStr := digestutil.Format(digest)
		pieceLink := testutil.RandomPiece(t, int64(len(data)))
		pieceLinkStr := pieceLink.Link().String()

		finder := mockPieceFinder{map[string]piece.PieceLink{digestStr: pieceLink}}
		reader := mockPieceReader{map[string][]byte{pieceLinkStr: data}}
		sizer := mockSizer{map[string]uint64{digestStr: uint64(len(data))}}

		getter := adapter.NewBlobGetterAdapter(&finder, &reader, &sizer)
		end := uint64(1)
		obj, err := getter.Get(t.Context(), digest, blobstore.WithRange(0, &end))
		require.NoError(t, err)

		require.Equal(t, int64(len(data)), obj.Size())
		require.Equal(t, data[0:2], testutil.Must(io.ReadAll(obj.Body()))(t))
	})
}

type mockPieceFinder struct {
	data map[string]piece.PieceLink
}

func (m *mockPieceFinder) FindPiece(ctx context.Context, digest multihash.Multihash, size uint64) (piece.PieceLink, error) {
	l, ok := m.data[digestutil.Format(digest)]
	if !ok {
		return nil, errors.New("not found")
	}
	return l, nil
}

func (m *mockPieceFinder) URLForPiece(ctx context.Context, piece piece.PieceLink) (url.URL, error) {
	panic("unimplemented")
}

var _ piecefinder.PieceFinder = (*mockPieceFinder)(nil)

type mockPieceReader struct {
	data map[string][]byte
}

func (m *mockPieceReader) ReadPiece(ctx context.Context, piece cid.Cid, options ...types.ReadPieceOption) (*types.PieceReader, error) {
	cfg := types.ReadPieceConfig{}
	cfg.ProcessOptions(options)

	data, ok := m.data[piece.String()]
	if !ok {
		return nil, errors.New("not found")
	}
	start := int(cfg.ByteRange.Start)
	end := len(data)
	if cfg.ByteRange.End != nil {
		end = int(*cfg.ByteRange.End + 1)
	}
	return &types.PieceReader{
		Size: int64(len(data)),
		Data: io.NopCloser(bytes.NewReader(data[start:end])),
	}, nil
}

var _ piecereader.PieceReader = (*mockPieceReader)(nil)

type mockSizer struct {
	data map[string]uint64
}

func (m *mockSizer) Size(ctx context.Context, digest multihash.Multihash) (uint64, error) {
	n, ok := m.data[digestutil.Format(digest)]
	if !ok {
		return 0, errors.New("not found")
	}
	return n, nil
}
