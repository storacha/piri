package testing

import (
	"testing"

	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateProofSet(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	harness, svc := SetupTestDeps(t, ctx, ctrl)

	err := svc.Start(ctx)
	require.NoError(t, err)

	defer func() {
		if err := svc.Stop(ctx); err != nil {
			t.Logf("failed to stop service: %v", err)
		}
	}()

	proofSetID := uint64(1)
	challengeWindow := uint64(30)
	provingPeriod := uint64(60)

	harness.CreateProofSet(ctx, svc, proofSetID, challengeWindow, provingPeriod)
}

func TestUploadPiece(t *testing.T) {
	logging.SetAllLoggers(logging.LevelInfo)
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	harness, svc := SetupTestDeps(t, ctx, ctrl)

	err := svc.Start(ctx)
	require.NoError(t, err)

	defer func() {
		if err := svc.Stop(ctx); err != nil {
			t.Logf("failed to stop service: %v", err)
		}
	}()

	proofSetID := uint64(1)
	challengeWindow := uint64(30)
	provingPeriod := uint64(60)

	harness.CreateProofSet(ctx, svc, proofSetID, challengeWindow, provingPeriod)
	harness.UploadPiece(ctx, svc, RandomBytes(t, 200), "")
}
