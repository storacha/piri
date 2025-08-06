package serve

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
)

var FXPDPCmd = &cobra.Command{
	Use:   "full-pdp",
	Args:  cobra.NoArgs,
	Short: `Start a PDP server`,
	RunE:  doFXPDPServe,
}

func init() {
	FXPDPCmd.Flags().String(
		"endpoint",
		config.DefaultPDPServer.Endpoint,
		"Endpoint for PDP server")
	cobra.CheckErr(viper.BindPFlag("endpoint", FXPDPCmd.Flags().Lookup("endpoint")))

	FXPDPCmd.Flags().String(
		"lotus-endpoint",
		"",
		"A websocket url for lotus node",
	)
	cobra.CheckErr(viper.BindPFlag("lotus_endpoint", FXPDPCmd.Flags().Lookup("lotus-endpoint")))

	FXPDPCmd.Flags().String(
		"owner-address",
		"",
		"The ethereum address to submit PDP Proofs with (must be in piri wallet - see `piri wallet` command for help",
	)
	cobra.CheckErr(viper.BindPFlag("owner_address", FXPDPCmd.Flags().Lookup("owner-address")))

	FXPDPCmd.Flags().String(
		"contract-address",
		"0x6170dE2b09b404776197485F3dc6c968Ef948505",
		"The ethereum address of the PDP contract",
	)
	cobra.CheckErr(viper.BindPFlag("contract_address", FXPDPCmd.Flags().Lookup("contract-address")))
}

func doFXPDPServe(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// load and validate the PDPServer configuration, applying all flags, env vars, and config file to config.
	// Failing if a required field is not present
	val := cmd.Flags().Lookup("lotus-url")
	_ = val
	cfg, err := config.Load[config.Piri]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := cfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	fxApp := fx.New(
		fx.Supply(appCfg),
		app.PDPServiceModule(appCfg),
		app.UCANServiceModule(appCfg),
	)
	if err := fxApp.Err(); err != nil {
		return err
	}
	// Start the application
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting fx app: %w", err)
	}

	// Wait for interrupt signal
	<-fxApp.Done()

	// Stop the application
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := fxApp.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping fx app: %w", err)
	}

	return nil
}
