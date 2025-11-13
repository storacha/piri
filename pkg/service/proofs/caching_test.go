package proofs_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/result/failure"
	"github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/piri/pkg/service/proofs"
	"github.com/stretchr/testify/require"
)

func TestCachingProofsService(t *testing.T) {
	webService := testutil.WebService

	server, err := server.NewServer(
		webService,
		server.WithServiceMethod(
			access.GrantAbility,
			server.Provide(
				access.Grant,
				func(
					ctx context.Context,
					capability ucan.Capability[access.GrantCaveats],
					invocation invocation.Invocation,
					context server.InvocationContext,
				) (result.Result[access.GrantOk, failure.IPLDBuilderFailure], fx.Effects, error) {
					nb := capability.Nb()
					dlg, err := delegation.Delegate(
						webService,
						invocation.Issuer(),
						[]ucan.Capability[ucan.NoCaveats]{
							ucan.NewCapability(nb.Att[0].Can, webService.DID().String(), ucan.NoCaveats{}),
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
	)
	require.NoError(t, err)

	conn, err := client.NewConnection(webService, server)
	require.NoError(t, err)

	proofsService := proofs.NewCachingProofService(testutil.Alice)

	ability := "test/test"
	dlg, err := proofsService.RequestAccess(t.Context(), webService, ability, nil, proofs.WithConnection(conn))
	require.NoError(t, err)

	require.Len(t, dlg.Capabilities(), 1)
	require.Equal(t, ability, dlg.Capabilities()[0].Can())
	require.Equal(t, webService.DID().String(), dlg.Capabilities()[0].With())

	// delegation should be cached
	cacheDlg, err := proofsService.RequestAccess(t.Context(), webService, ability, nil, proofs.WithConnection(conn))
	require.NoError(t, err)

	// if nonce is different it went back to the server
	require.Equal(t, dlg.Nonce(), cacheDlg.Nonce())

	// should get a fresh one if existing TTL is less than passed minimum
	freshDlg, err := proofsService.RequestAccess(
		t.Context(),
		webService,
		ability,
		nil,
		proofs.WithConnection(conn),
		proofs.WithMinimumTTL(time.Hour),
	)
	require.NoError(t, err)
	require.NotEqual(t, dlg.Nonce(), freshDlg.Nonce())
}
