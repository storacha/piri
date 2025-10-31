package storage_test

import (
	"testing"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/capabilities/access"
	"github.com/storacha/go-libstoracha/capabilities/assert"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/types"
	"github.com/storacha/go-libstoracha/testutil"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/transport/http"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"github.com/storacha/piri/pkg/fx/app"
	piritestutil "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/principalresolver"
	"github.com/storacha/piri/pkg/service/storage"
	strucan "github.com/storacha/piri/pkg/service/storage/ucan"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestFXAccessGrant(t *testing.T) {
	var svc storage.Service

	granter := testutil.Alice
	strnde := testutil.Bob
	idxsvc := testutil.Mallory
	upsvc := testutil.WebService

	appConfig := piritestutil.NewTestConfig(
		t,
		piritestutil.WithSigner(granter),
		piritestutil.WithUploadServiceConfig(upsvc.DID(), testutil.TestURL),
	)
	testApp := fxtest.New(t,
		fx.NopLogger,
		app.CommonModules(appConfig),
		app.UCANModule,
		// use the map resolver so no network calls are made that would fail anyway
		fx.Decorate(func() validator.PrincipalResolver {
			return testutil.Must(principalresolver.NewMapResolver(map[string]string{
				upsvc.DID().String(): upsvc.Unwrap().DID().String(),
			}))(t)
		}),
		fx.Populate(&svc),
	)

	testApp.RequireStart()
	defer testApp.RequireStop()
	piritestutil.WaitForHealthy(t, &appConfig.Server.PublicURL)

	channel := http.NewChannel(&appConfig.Server.PublicURL)
	conn, err := client.NewConnection(granter, channel)
	require.NoError(t, err)

	cid := testutil.RandomCID(t)
	digest := testutil.RandomMultihash(t)

	testCases := []struct {
		name         string
		granter      ucan.Signer
		grantee      ucan.Signer
		ability      ucan.Ability
		cause        invocation.Invocation
		expectDigest multihash.Multihash
		expectError  string
	}{
		{
			name:    "grant blob/retrieve for blob/replica/allocate",
			granter: granter,
			grantee: strnde,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				replica.Allocate.Invoke(
					upsvc,
					strnde,
					strnde.DID().String(),
					replica.AllocateCaveats{
						Space: testutil.RandomDID(t),
						Blob: types.Blob{
							Digest: digest,
							Size:   1234,
						},
						Site:  testutil.RandomCID(t),
						Cause: testutil.RandomCID(t),
					},
					delegation.WithProof(
						delegation.FromDelegation(
							testutil.Must(
								delegation.Delegate(
									strnde,
									upsvc,
									[]ucan.Capability[ucan.NoCaveats]{
										ucan.NewCapability(replica.AllocateAbility, strnde.DID().String(), ucan.NoCaveats{}),
									},
								),
							)(t),
						),
					),
				),
			)(t),
			expectDigest: digest,
		},
		{
			name:    "unauthorized blob/replica/allocate cause",
			granter: granter,
			grantee: strnde,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				replica.Allocate.Invoke(
					upsvc,
					strnde,
					strnde.DID().String(),
					replica.AllocateCaveats{
						Space: testutil.RandomDID(t),
						Blob: types.Blob{
							Digest: digest,
							Size:   1234,
						},
						Site:  testutil.RandomCID(t),
						Cause: testutil.RandomCID(t),
					},
				),
			)(t),
			expectError: strucan.UnauthorizedCauseErrorName,
		},
		{
			name:    "grant blob/retrieve for assert/index",
			granter: granter,
			grantee: idxsvc,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				assert.Index.Invoke(
					upsvc,
					idxsvc,
					idxsvc.DID().String(),
					assert.IndexCaveats{
						Content: testutil.RandomCID(t),
						Index:   cid,
					},
					delegation.WithProof(
						delegation.FromDelegation(
							testutil.Must(
								delegation.Delegate(
									idxsvc,
									upsvc,
									[]ucan.Capability[ucan.NoCaveats]{
										ucan.NewCapability(assert.IndexAbility, idxsvc.DID().String(), ucan.NoCaveats{}),
									},
								),
							)(t),
						),
					),
				),
			)(t),
			expectDigest: cid.(cidlink.Link).Hash(),
		},
		{
			name:    "unauthorized assert/index cause",
			granter: granter,
			grantee: idxsvc,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				assert.Index.Invoke(
					upsvc,
					idxsvc,
					idxsvc.DID().String(),
					assert.IndexCaveats{
						Content: testutil.RandomCID(t),
						Index:   cid,
					},
				),
			)(t),
			expectError: strucan.UnauthorizedCauseErrorName,
		},
		{
			name:    "cause audience mismatch",
			granter: granter,
			grantee: idxsvc,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				replica.Allocate.Invoke(
					upsvc,
					strnde,
					strnde.DID().String(),
					replica.AllocateCaveats{
						Space: testutil.RandomDID(t),
						Blob: types.Blob{
							Digest: digest,
							Size:   1234,
						},
						Site:  testutil.RandomCID(t),
						Cause: testutil.RandomCID(t),
					},
					delegation.WithProof(
						delegation.FromDelegation(
							testutil.Must(
								delegation.Delegate(
									strnde,
									upsvc,
									[]ucan.Capability[ucan.NoCaveats]{
										ucan.NewCapability(replica.AllocateAbility, strnde.DID().String(), ucan.NoCaveats{}),
									},
								),
							)(t),
						),
					),
				),
			)(t),
			expectError: strucan.InvalidCauseErrorName,
		},
		{
			name:    "cause issuer mismatch",
			granter: granter,
			grantee: strnde,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				replica.Allocate.Invoke(
					strnde,
					strnde,
					strnde.DID().String(),
					replica.AllocateCaveats{
						Space: testutil.RandomDID(t),
						Blob: types.Blob{
							Digest: digest,
							Size:   1234,
						},
						Site:  testutil.RandomCID(t),
						Cause: testutil.RandomCID(t),
					},
				),
			)(t),
			expectError: strucan.InvalidCauseErrorName,
		},
		{
			name:        "request grant for unknown capability",
			granter:     granter,
			grantee:     strnde,
			ability:     "unknown/ability",
			expectError: strucan.UnknownAbilityErrorName,
		},
		{
			name:    "unknown cause invocation",
			granter: granter,
			grantee: strnde,
			ability: blob.RetrieveAbility,
			cause: testutil.Must(
				assert.Equals.Invoke(
					upsvc,
					strnde,
					strnde.DID().String(),
					assert.EqualsCaveats{
						Content: types.FromHash(digest),
						Equals:  testutil.RandomCID(t),
					},
					delegation.WithProof(
						delegation.FromDelegation(
							testutil.Must(
								delegation.Delegate(
									strnde,
									upsvc,
									[]ucan.Capability[ucan.NoCaveats]{
										ucan.NewCapability(assert.EqualsAbility, strnde.DID().String(), ucan.NoCaveats{}),
									},
								),
							)(t),
						),
					),
				),
			)(t),
			expectError: strucan.UnknownCauseErrorName,
		},
		{
			name:        "missing cause",
			granter:     granter,
			grantee:     strnde,
			ability:     blob.RetrieveAbility,
			expectError: strucan.MissingCauseErrorName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nb := access.GrantCaveats{
				Att: []access.CapabilityRequest{{Can: tc.ability}},
			}
			if tc.cause != nil {
				nb.Cause = tc.cause.Link()
			}

			inv, err := access.Grant.Invoke(tc.grantee, tc.granter, tc.grantee.DID().String(), nb)
			require.NoError(t, err)

			if tc.cause != nil {
				for b, err := range tc.cause.Export() {
					require.NoError(t, err)
					inv.Attach(b)
				}
			}

			xres, err := client.Execute(t.Context(), []invocation.Invocation{inv}, conn)
			require.NoError(t, err)

			rcptLink, ok := xres.Get(inv.Link())
			require.True(t, ok)

			rcptReader, err := access.NewGrantReceiptReader()
			require.NoError(t, err)

			rcpt, err := rcptReader.Read(rcptLink, xres.Blocks())
			require.NoError(t, err)

			o, x := result.Unwrap(rcpt.Out())

			if tc.expectError != "" {
				require.Empty(t, o)
				t.Logf("%s: %s", x.Name(), x.Error())
				require.Equal(t, tc.expectError, x.Name())
			} else {
				require.Empty(t, x)
				require.Len(t, o.Delegations.Values, 1)

				dlgBytes := o.Delegations.Values[o.Delegations.Keys[0]]
				dlg, err := delegation.Extract(dlgBytes)
				require.NoError(t, err)
				require.Equal(t, blob.RetrieveAbility, dlg.Capabilities()[0].Can())

				match, err := blob.Retrieve.Match(validator.NewSource(dlg.Capabilities()[0], dlg))
				require.NoError(t, err)
				require.Equal(t, tc.expectDigest, match.Value().Nb().Blob.Digest)
			}
		})
	}
}
