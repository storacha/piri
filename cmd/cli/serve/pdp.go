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
		"lotus-url",
		"",
		"A websocket url for lotus node",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.lotus_endpoint", PDPCmd.Flags().Lookup("lotus-url")))

	PDPCmd.Flags().String(
		"owner-address",
		"",
		"The ethereum address to submit PDP Proofs with (must be in piri wallet - see `piri wallet` command for help)",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.owner_address", PDPCmd.Flags().Lookup("owner-address")))

	PDPCmd.Flags().String(
		"contract-address",
		"0x6170dE2b09b404776197485F3dc6c968Ef948505", // NB(forrest): default to calibration contract addrese
		"The ethereum address of the PDP Contract",
	)
	cobra.CheckErr(viper.BindPFlag("pdp.contract_address", PDPCmd.Flags().Lookup("contract-address")))
}

func doPDPServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// load and validate the PDPServer configuration, applying all flags, env vars, and config file to config.
	// Failing if a required field is not present
	cfg, err := config.Load[config.PDPServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := os.MkdirAll(cfg.Repo.DataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %s: %w", cfg.Repo.DataDir, err)
	}

	walletDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "wallet")
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

	dataDir, err := cliutil.Mkdirp(cfg.Repo.DataDir, "pdp")
	if err != nil {
		return err
	}

	// parse server endpoint to serve on, must be http as piri doesn't support tls termination
	serverEndpoint, err := url.Parse(fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port))
	if err != nil {
		return fmt.Errorf("invalid server endpoint %s: %w", serverEndpoint, err)
	}
	if serverEndpoint.Scheme != "http" {
		return fmt.Errorf("invalid endpoint %s: must use http", serverEndpoint)
	}

	// parse the lotus endpoint
	lotusEndpoint, err := url.Parse(cfg.PDPService.LotusEndpoint)
	if err != nil {
		return fmt.Errorf("invalid lotus endpoint %s: %w", cfg.PDPService.LotusEndpoint, err)
	}

	// parse the users owner address, used to send message on chain
	if !common.IsHexAddress(cfg.PDPService.OwnerAddress) {
		return fmt.Errorf("invalid eth address: %s", cfg.PDPService.OwnerAddress)
	}
	ownerAddress := common.HexToAddress(cfg.PDPService.OwnerAddress)
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
		DataDir:      cfg.Repo.DataDir,
	}
	cliutil.PrintPDPServerConfig(cmd, serverConfig)

	if err := svr.Start(ctx); err != nil {
		return fmt.Errorf("starting pdp server: %w", err)
	}

	cmd.Printf("Server started! Listening on %s\n", serverEndpoint.String())

	// publish version info metric
	telemetry.RecordServerInfo(ctx, "pdp", telemetry.StringAttr("eth_address", cfg.PDPService.OwnerAddress))

	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := svr.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping pdp server: %w", err)
	}
	return nil
}
