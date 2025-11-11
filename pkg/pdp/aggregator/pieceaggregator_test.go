package aggregator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/piri/internal/mocks"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/aggregator/aggregate"
	"github.com/storacha/piri/pkg/pdp/aggregator/fns"
)

type fakeQueue struct {
	submittedLinks []datamodel.Link
}

func (f *fakeQueue) Enqueue(ctx context.Context, _ string, link datamodel.Link) error {
	f.submittedLinks = append(f.submittedLinks, link)
	return nil
}

func setupPieceAggregatorDependencies(
	ctrl *gomock.Controller,
) (
	*mocks.MockInProgressWorkspace,
	*mocks.MockKVStore[datamodel.Link, aggregate.Aggregate],
	*fakeQueue,
	*mocks.MockBufferedAggregator,
) {
	workspaceMock := mocks.NewMockInProgressWorkspace(ctrl)
	storeMock := mocks.NewMockKVStore[datamodel.Link, aggregate.Aggregate](ctrl)
	baMock := mocks.NewMockBufferedAggregator(ctrl)

	return workspaceMock, storeMock, &fakeQueue{submittedLinks: make([]datamodel.Link, 0)}, baMock
}

const (
	MB = 1 << 20
)

func TestPieceAggregator_StoreAndSubmit(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock, storeMock, queueSubMock, baMock := setupPieceAggregatorDependencies(ctrl)

	pa := aggregator.NewAggregatorHandler(workspaceMock, storeMock, queueSubMock, aggregator.WithAggregatorHandler(baMock))
	// the below makes assertion that when three aggregates are returned by the aggregator of the piece-aggregator
	// three writes are made to the aggregate-store and three submissions are made to the queue-submission function.
	p1 := testutil.RandomPiece(t, MB)
	p2 := testutil.RandomPiece(t, MB)
	p3 := testutil.RandomPiece(t, MB)
	expectedPieces := []piece.PieceLink{p1, p2, p3}
	expectedAggregates := []aggregate.Aggregate{{Root: p1}, {Root: p2}, {Root: p3}}
	expectedBuffer := fns.Buffer{}

	workspaceMock.EXPECT().GetBuffer(ctx).Return(expectedBuffer, nil)
	baMock.EXPECT().AggregatePieces(expectedBuffer, expectedPieces).Return(expectedBuffer, expectedAggregates, nil)
	workspaceMock.EXPECT().PutBuffer(ctx, expectedBuffer).Return(nil)
	storeMock.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil)
	storeMock.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil)
	storeMock.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil)

	err := pa.Handle(ctx, expectedPieces)
	require.NoError(t, err)

	require.Len(t, queueSubMock.submittedLinks, len(expectedAggregates))
}

func TestPieceAggregator_GetBufferError(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock, storeMock, queueSubMock, baMock := setupPieceAggregatorDependencies(ctrl)

	pa := aggregator.NewAggregatorHandler(workspaceMock, storeMock, queueSubMock, aggregator.WithAggregatorHandler(baMock))
	workspaceMock.EXPECT().GetBuffer(ctx).Return(fns.Buffer{}, fmt.Errorf("get buffer error"))

	err := pa.Handle(ctx, nil)
	require.Error(t, err)

	require.Len(t, queueSubMock.submittedLinks, 0)
}

func TestPieceAggregator_PutBufferError(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock, storeMock, queueSubMock, baMock := setupPieceAggregatorDependencies(ctrl)

	pa := aggregator.NewAggregatorHandler(workspaceMock, storeMock, queueSubMock, aggregator.WithAggregatorHandler(baMock))
	p1 := testutil.RandomPiece(t, MB)
	p2 := testutil.RandomPiece(t, MB)
	p3 := testutil.RandomPiece(t, MB)
	expectedPieces := []piece.PieceLink{p1, p2, p3}
	expectedAggregates := []aggregate.Aggregate{{Root: p1}, {Root: p2}, {Root: p3}}
	expectedBuffer := fns.Buffer{}

	workspaceMock.EXPECT().GetBuffer(ctx).Return(expectedBuffer, nil)
	baMock.EXPECT().AggregatePieces(expectedBuffer, expectedPieces).Return(expectedBuffer, expectedAggregates, nil)
	workspaceMock.EXPECT().PutBuffer(ctx, expectedBuffer).Return(fmt.Errorf("put buffer error"))

	err := pa.Handle(ctx, expectedPieces)
	require.Error(t, err)

	require.Len(t, queueSubMock.submittedLinks, 0)
}

func TestPieceAggregator_AggregatePieceError(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock, storeMock, queueSubMock, baMock := setupPieceAggregatorDependencies(ctrl)

	pa := aggregator.NewAggregatorHandler(workspaceMock, storeMock, queueSubMock, aggregator.WithAggregatorHandler(baMock))
	p1 := testutil.RandomPiece(t, MB)
	p2 := testutil.RandomPiece(t, MB)
	p3 := testutil.RandomPiece(t, MB)
	expectedPieces := []piece.PieceLink{p1, p2, p3}
	expectedBuffer := fns.Buffer{}

	workspaceMock.EXPECT().GetBuffer(ctx).Return(expectedBuffer, nil)
	baMock.EXPECT().AggregatePieces(expectedBuffer, expectedPieces).Return(expectedBuffer, nil, fmt.Errorf("aggregate piece error"))

	err := pa.Handle(ctx, expectedPieces)
	require.Error(t, err)

	require.Len(t, queueSubMock.submittedLinks, 0)
}

func TestPieceAggregator_StorePutError(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock, storeMock, queueSubMock, baMock := setupPieceAggregatorDependencies(ctrl)

	pa := aggregator.NewAggregatorHandler(workspaceMock, storeMock, queueSubMock, aggregator.WithAggregatorHandler(baMock))
	p1 := testutil.RandomPiece(t, MB)
	p2 := testutil.RandomPiece(t, MB)
	p3 := testutil.RandomPiece(t, MB)
	expectedPieces := []piece.PieceLink{p1, p2, p3}
	expectedBuffer := fns.Buffer{}
	expectedAggregates := []aggregate.Aggregate{{Root: p1}, {Root: p2}, {Root: p3}}

	workspaceMock.EXPECT().GetBuffer(ctx).Return(expectedBuffer, nil)
	baMock.EXPECT().AggregatePieces(expectedBuffer, expectedPieces).Return(expectedBuffer, expectedAggregates, nil)
	workspaceMock.EXPECT().PutBuffer(ctx, expectedBuffer).Return(nil)
	storeMock.EXPECT().Put(ctx, expectedAggregates[0].Root.Link(), expectedAggregates[0]).Return(fmt.Errorf("put buffer error"))

	err := pa.Handle(ctx, expectedPieces)
	require.Error(t, err)

	require.Len(t, queueSubMock.submittedLinks, 0)
}
