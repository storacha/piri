package signer_test

import (
	"context"
	"encoding/hex"
	"io"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/storacha/filecoin-services/go/eip712"
	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/pdp/sign"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/principal"
	ucan_server "github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/service/proofs"
	signerclient "github.com/storacha/piri/pkg/service/signer"
	"github.com/stretchr/testify/require"
)

func TestProofServiceSigner(t *testing.T) {
	signerServiceID := testutil.WebService
	server := mockSigningServiceServer(t, signerServiceID)

	conn, err := client.NewConnection(signerServiceID, server)
	require.NoError(t, err)

	proofService := proofs.NewCachingProofService()
	signingService := signerclient.NewProofServiceSigner(conn, proofService)

	t.Run("pdp/sign/dataset/create", func(t *testing.T) {
		payee := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb")
		_, err := signingService.SignCreateDataSet(
			t.Context(),
			testutil.Alice,
			testutil.RandomBigInt(t),
			payee,
			[]eip712.MetadataEntry{
				{Key: "name", Value: "test-dataset"},
				{Key: "version", Value: "1.0"},
			},
		)
		require.NoError(t, err)
	})

	t.Run("pdp/sign/dataset/delete", func(t *testing.T) {
		_, err := signingService.SignDeleteDataSet(t.Context(), testutil.Alice, testutil.RandomBigInt(t))
		require.NoError(t, err)
	})

	t.Run("pdp/sign/pieces/add", func(t *testing.T) {
		_, err := signingService.SignAddPieces(
			t.Context(),
			testutil.Alice,
			testutil.RandomBigInt(t),
			big.NewInt(0),
			[][]byte{
				testutil.Must(hex.DecodeString("0001020304"))(t),
				testutil.Must(hex.DecodeString("0506070809"))(t),
			},
			[][]eip712.MetadataEntry{
				{{Key: "size", Value: "1024"}},
				{{Key: "size", Value: "2048"}},
			},
			[][]receipt.AnyReceipt{},
		)
		require.NoError(t, err)
	})

	t.Run("pdp/sign/pieces/remove/schedule", func(t *testing.T) {
		_, err := signingService.SignSchedulePieceRemovals(
			t.Context(),
			testutil.Alice,
			testutil.RandomBigInt(t),
			[]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
		)
		require.NoError(t, err)
	})
}

func mockSigningServiceServer(t *testing.T, id principal.Signer) ucan_server.ServerView[ucan_server.Service] {
	mockSignature := mockSignature()
	server, err := ucan_server.NewServer(
		id,
		ucan_server.WithServiceMethod(
			access.GrantAbility,
			ucan_server.Provide(
				access.Grant,
				func(
					ctx context.Context,
					capability ucan.Capability[access.GrantCaveats],
					invocation invocation.Invocation,
					context ucan_server.InvocationContext,
				) (result.Result[access.GrantOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					nb := capability.Nb()
					dlg, err := delegation.Delegate(
						id,
						invocation.Issuer(),
						[]ucan.Capability[ucan.NoCaveats]{
							ucan.NewCapability(nb.Att[0].Can, id.DID().String(), ucan.NoCaveats{}),
						},
						delegation.WithExpiration(ucan.Now()+30),
						delegation.WithNonce(testutil.RandomCID(t).String()),
					)
					require.NoError(t, err)

					dlgArchive := testutil.Must(io.ReadAll(dlg.Archive()))(t)

					return result.Ok[access.GrantOk, failure.IPLDBuilderFailure](
						access.GrantOk{
							Delegations: access.DelegationsModel{
								Keys:   []string{dlg.Link().String()},
								Values: map[string][]byte{dlg.Link().String(): dlgArchive},
							},
						},
					), nil, nil
				},
			),
		),
		ucan_server.WithServiceMethod(
			sign.DataSetCreateAbility,
			ucan_server.Provide(
				sign.DataSetCreate,
				func(
					ctx context.Context,
					cap ucan.Capability[sign.DataSetCreateCaveats],
					inv invocation.Invocation,
					ictx ucan_server.InvocationContext,
				) (result.Result[sign.DataSetCreateOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					return result.Ok[sign.DataSetCreateOk, failure.IPLDBuilderFailure](sign.DataSetCreateOk(*mockSignature)), nil, nil
				},
			),
		),
		ucan_server.WithServiceMethod(
			sign.DataSetDeleteAbility,
			ucan_server.Provide(
				sign.DataSetDelete,
				func(
					ctx context.Context,
					cap ucan.Capability[sign.DataSetDeleteCaveats],
					inv invocation.Invocation,
					ictx ucan_server.InvocationContext,
				) (result.Result[sign.DataSetDeleteOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					return result.Ok[sign.DataSetDeleteOk, failure.IPLDBuilderFailure](sign.DataSetDeleteOk(*mockSignature)), nil, nil
				},
			),
		),
		ucan_server.WithServiceMethod(
			sign.PiecesAddAbility,
			ucan_server.Provide(
				sign.PiecesAdd,
				func(
					ctx context.Context,
					cap ucan.Capability[sign.PiecesAddCaveats],
					inv invocation.Invocation,
					ictx ucan_server.InvocationContext,
				) (result.Result[sign.PiecesAddOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					return result.Ok[sign.PiecesAddOk, failure.IPLDBuilderFailure](sign.PiecesAddOk(*mockSignature)), nil, nil
				},
			),
		),
		ucan_server.WithServiceMethod(
			sign.PiecesRemoveScheduleAbility,
			ucan_server.Provide(
				sign.PiecesRemoveSchedule,
				func(
					ctx context.Context,
					cap ucan.Capability[sign.PiecesRemoveScheduleCaveats],
					inv invocation.Invocation,
					ictx ucan_server.InvocationContext,
				) (result.Result[sign.PiecesRemoveScheduleOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					return result.Ok[sign.PiecesRemoveScheduleOk, failure.IPLDBuilderFailure](sign.PiecesRemoveScheduleOk(*mockSignature)), nil, nil
				},
			),
		),
	)
	require.NoError(t, err)
	return server
}

func mockSignature() *eip712.AuthSignature {
	return &eip712.AuthSignature{
		Signer: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		R:      common.BigToHash(big.NewInt(12345)),
		S:      common.BigToHash(big.NewInt(67890)),
		V:      27,
	}
}
