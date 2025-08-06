package blockchain

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
)

var Module = fx.Module("blockchain",
	fx.Provide(
		fx.Annotate(
			func() *contract.PDPContract { return &contract.PDPContract{} },
			fx.As(new(contract.PDP)),
		),
		fx.Annotate(
			ProvideEthAPI,
			fx.As(new(service.EthClient)),
		),
		fx.Annotate(
			ProvideLotusAPI,
			fx.As(new(service.ChainClient)),
		),
	),
)

func ProvideEthAPI(lc fx.Lifecycle, cfg app.AppConfig) (*ethclient.Client, error) {
	ethAPI, err := ethclient.Dial(cfg.Blockchain.LotusEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("providing eth api: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			ethAPI.Close()
			return nil
		},
	})
	return ethAPI, nil
}

func ProvideLotusAPI(lc fx.Lifecycle, cfg app.AppConfig) (api.FullNode, error) {
	lotusAPI, closer, err := client.NewFullNodeRPCV1(context.TODO(), cfg.Blockchain.LotusEndpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("providing lotus api: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			closer()
			return nil
		},
	})
	return lotusAPI, nil
}
