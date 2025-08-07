package serve

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/telemetry"
	"github.com/storacha/piri/pkg/wallet"
)

var PDPCmd = &cobra.Command{
	Use:   "pdp",
	Args:  cobra.NoArgs,
	Short: `Start a PDP server`,
	RunE:  doPDPServe,
}

func init() {
	PDPCmd.Flags().String(
		"endpoint",
		config.DefaultPDPServer.Endpoint,
		"Endpoint for PDP server")
	cobra.CheckErr(viper.BindPFlag("endpoint", PDPCmd.Flags().Lookup("endpoint")))

	PDPCmd.Flags().String(
		"lotus-url",
		"",
		"A websocket url for lotus node",
	)
	cobra.CheckErr(viper.BindPFlag("lotus_url", PDPCmd.Flags().Lookup("lotus-url")))

	PDPCmd.Flags().String(
		"eth-address",
		"",
		"The ethereum address to submit PDP Proofs with (must be in piri wallet - see `piri wallet` command for help",
	)
	cobra.CheckErr(viper.BindPFlag("eth_address", PDPCmd.Flags().Lookup("eth-address")))
}

func doPDPServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// load and validate the PDPServer configuration, applying all flags, env vars, and config file to config.
	// Failing if a required field is not present
	cfg, err := config.Load[config.PDPServer]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %s: %w", cfg.DataDir, err)
	}

	walletDir, err := cliutil.Mkdirp(cfg.DataDir, "wallet")
	if err != nil {
		return err
	}

	walletDs, err := leveldb.NewDatastore(walletDir, nil)
	if err != nil {
		return err
	}

	keyStore, err := keystore.NewKeyStore(walletDs)
	if err != nil {
		return err
	}

	wlt, err := wallet.NewWallet(keyStore)
	if err != nil {
		return err
	}

	dataDir, err := cliutil.Mkdirp(cfg.DataDir, "pdp")
	if err != nil {
		return err
	}

	// parse server endpoint to serve on, must be http as piri doesn't support tls termination
	serverEndpoint, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid server endpoint %s: %w", cfg.Endpoint, err)
	}
	if serverEndpoint.Scheme != "http" {
		return fmt.Errorf("invalid endpoint %s: must use http", cfg.Endpoint)
	}

	// parse the lotus endpoint
	lotusEndpoint, err := url.Parse(cfg.LotusURL)
	if err != nil {
		return fmt.Errorf("invalid lotus endpoint %s: %w", cfg.LotusURL, err)
	}

	// parse the users owner address, used to send message on chain
	if !common.IsHexAddress(cfg.EthAddress) {
		return fmt.Errorf("invalid eth address: %s", cfg.EthAddress)
	}
	ownerAddress := common.HexToAddress(cfg.EthAddress)
	svr, err := pdp.NewServer(
		ctx,
		dataDir,
		serverEndpoint,
		lotusEndpoint,
		ownerAddress,
		wlt,
	)
	if err != nil {
		return fmt.Errorf("creating pdp server: %w", err)
	}

	serverConfig := cliutil.PDPServerConfig{
		Endpoint:     serverEndpoint,
		LotusURL:     lotusEndpoint,
		OwnerAddress: ownerAddress,
		DataDir:      cfg.DataDir,
	}
	cliutil.PrintPDPServerConfig(cmd, serverConfig)

	if err := svr.Start(ctx); err != nil {
		return fmt.Errorf("starting pdp server: %w", err)
	}

	cmd.Println("Server started! Listening on ", cfg.Endpoint)

	// publish version info metric
	telemetry.RecordServerInfo(ctx, "pdp", telemetry.StringAttr("eth_address", cfg.EthAddress))

	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := svr.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping pdp server: %w", err)
	}
	return nil
}
