package app

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/aggregator"
	"github.com/storacha/piri/pkg/fx/pdp"
	"github.com/storacha/piri/pkg/fx/scheduler"
	"github.com/storacha/piri/pkg/fx/wallet"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/service"
)

var PDPModule = fx.Module("pdp",
	fx.Provide(
		fx.Annotate(
			ProvideContractClient,
			// provide the contract as it's interface
			fx.As(new(smartcontracts.PDP)),
		),
		fx.Annotate(
			ProvideEthClient,
			// provide as interface required by service(s)
			fx.As(new(service.EthClient)),
		),
		fx.Annotate(
			ProvideLotusClient,
			// provide as interface required by service(s)
			fx.As(new(service.ChainClient)),
		),
	),
	aggregator.Module,
	scheduler.Module,
	pdp.Module,
	wallet.Module,
)

func ProvideContractClient() *smartcontracts.PDPContract {
	return new(smartcontracts.PDPContract)
}

func ProvideEthClient(lc fx.Lifecycle, cfg app.AppConfig) (*ethclient.Client, error) {
	ethAPI, err := ethclient.Dial(cfg.PDPService.LotusEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("providing eth client: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			ethAPI.Close()
			return nil
		},
	})
	return ethAPI, nil
}

func ProvideLotusClient(lc fx.Lifecycle, cfg app.AppConfig) (api.FullNode, error) {
	lotusAPI, closer, err := client.NewFullNodeRPCV1(context.TODO(), cfg.PDPService.LotusEndpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("providing lotus client: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			closer()
			return nil
		},
	})
	return lotusAPI, nil
}
