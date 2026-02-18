package testutil

import (
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/multiformats/go-multiaddr"
	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/config"
	appcfg "github.com/storacha/piri/pkg/config/app"
	appfx "github.com/storacha/piri/pkg/fx/app"
	testutil2 "github.com/storacha/piri/pkg/internal/testutil"
	"github.com/storacha/piri/pkg/pdp/types"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/testutil/localdev"
	"github.com/storacha/piri/pkg/wallet"
)

// NodeInfo contains information about a test node needed for client creation.
type NodeInfo struct {
	API      types.API
	URL      url.URL
	Signer   principal.Signer
	EngineDB *gorm.DB
}

func NewNode(t testing.TB, container *localdev.Container, signer principal.Signer) (*NodeInfo, func()) {
	t.Helper()
	dataDir := t.TempDir()
	tempDir := t.TempDir()

	repoCfg := config.RepoConfig{
		DataDir: dataDir,
		TempDir: tempDir,
	}
	strgCfg, err := repoCfg.ToAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	port := testutil2.GetFreePort(t)
	publicURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	require.NoError(t, err)

	wsEndpoint, err := url.Parse(strings.Replace(container.RPCEndpoint, "http://", "ws://", 1))
	require.NoError(t, err)

	payerKey, err := crypto.HexToECDSA(strings.TrimPrefix(localdev.Accounts.Payer.PrivateKey, "0x"))
	require.NoError(t, err)

	ownerKey, err := crypto.HexToECDSA(strings.TrimPrefix(localdev.Accounts.ServiceProvider.PrivateKey, "0x"))
	require.NoError(t, err)

	var api types.API
	var engineDB *gorm.DB
	piri := fxtest.New(t,
		fx.Populate(&api),
		fx.NopLogger,
		fx.Populate(fx.Annotate(&engineDB, fx.ParamTags(`name:"engine_db"`))),
		appfx.CommonModules(appcfg.AppConfig{
			Identity: appcfg.IdentityConfig{
				Signer: signer,
			},
			Server: appcfg.ServerConfig{
				Host:      "localhost",
				Port:      uint(port),
				PublicURL: *publicURL,
			},
			Storage: strgCfg,
			UCANService: appcfg.UCANServiceConfig{
				Services: appcfg.ExternalServicesConfig{
					PrincipalMapping: map[string]string{},
					Upload: appcfg.UploadServiceConfig{
						Connection: lo.Must(
							client.NewConnection(
								lo.Must(did.Parse("did:web:up.test.storacha.network")),
								ucanhttp.NewChannel(lo.Must(url.Parse("http://up.test.storacha.network"))),
							),
						),
					},
					Publisher: appcfg.PublisherServiceConfig{
						PublicMaddr:   lo.Must(multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/http", port))),
						AnnounceMaddr: lo.Must(multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/http", port))),
						AnnounceURLs:  []url.URL{}, // Empty by default for tests
					},
				},
				ProofSetID: 1,
			},
			PDPService: appcfg.PDPServiceConfig{
				OwnerAddress:  common.HexToAddress(localdev.Accounts.ServiceProvider.Address),
				LotusEndpoint: wsEndpoint,
				SigningService: appcfg.SigningServiceConfig{
					PrivateKey: payerKey,
				},
				Contracts: appcfg.ContractAddresses{
					Verifier:         common.HexToAddress(container.Addresses.PDPVerifier),
					ProviderRegistry: common.HexToAddress(container.Addresses.ServiceProviderRegistry),
					Service:          common.HexToAddress(container.Addresses.FilecoinWarmStorageService),
					ServiceView:      common.HexToAddress(container.Addresses.ServiceStateView),
				},
				ChainID:      big.NewInt(localdev.ChainID),
				PayerAddress: common.HexToAddress(localdev.Accounts.Payer.Address),
			},
			Replicator: appcfg.ReplicatorConfig{
				MaxRetries: 1,
				MaxWorkers: 1,
				MaxTimeout: time.Second,
			},
		}),
		appfx.UCANModule,
		appfx.PDPModule,
		fx.Decorate(func(wlt wallet.Wallet) wallet.Wallet {
			if _, err := wlt.Import(t.Context(), &keystore.KeyInfo{PrivateKey: crypto.FromECDSA(ownerKey)}); err != nil {
				t.Fatal(err)
			}
			return wlt
		}),
	)
	piri.RequireStart()

	return &NodeInfo{
		API:      api,
		URL:      *publicURL,
		Signer:   signer,
		EngineDB: engineDB,
	}, piri.RequireStop
}
