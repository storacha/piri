package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/storacha/piri/delegator/internal/config"
	"github.com/storacha/piri/delegator/internal/handlers"
	"github.com/storacha/piri/delegator/internal/providers"
	"github.com/storacha/piri/delegator/internal/server"
	"github.com/storacha/piri/delegator/internal/service"
	"github.com/storacha/piri/delegator/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	Long:  `Start the delegator HTTP server with configured endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app := fx.New(
			fx.Provide(
				// Configuration
				config.NewConfig,

				func(cfg *config.Config) config.DynamoConfig {
					return cfg.Store
				},

				// Providers for complex types
				providers.ProvideSigner,
				providers.ProvideIndexingServiceWebDID,
				providers.ProvideIndexingServiceProof,
				providers.ProvideUploadServiceDID,

				// Store
				fx.Annotate(
					store.NewDynamoDBStore,
					fx.As(new(store.Store)),
				),

				// Service
				service.NewDelegatorService,

				// Handlers and Server
				handlers.NewHandlers,
				server.NewServer,
			),
			fx.Invoke(server.Start),
		)

		app.Run()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server flags
	serveCmd.Flags().String("host", "0.0.0.0", "Server host")
	serveCmd.Flags().Int("port", 8080, "Server port")

	// Store flags
	serveCmd.Flags().String("store-region", "", "AWS region for DynamoDB")
	serveCmd.Flags().String("store-allowlist-table", "", "DynamoDB table name for allowlist")
	serveCmd.Flags().String("store-providerinfo-table", "", "DynamoDB table name for provider info")
	serveCmd.Flags().Uint("store-provider-weight", 1, "Default weight for registered providers")
	serveCmd.Flags().String("store-endpoint", "", "DynamoDB endpoint (for local testing)")

	// Delegator flags
	serveCmd.Flags().String("delegator-key-file", "", "Path to delegator private key file")
	serveCmd.Flags().String("delegator-indexing-service-did", "", "DID of the indexing service")
	serveCmd.Flags().String("delegator-indexing-service-proof", "", "Path to proof file from indexing service")
	serveCmd.Flags().String("delegator-upload-service-did", "", "DID of the upload service")

	// Bind flags to viper
	viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("store.region", serveCmd.Flags().Lookup("store-region"))
	viper.BindPFlag("store.allowlist_table_name", serveCmd.Flags().Lookup("store-allowlist-table"))
	viper.BindPFlag("store.providerinfo_table_name", serveCmd.Flags().Lookup("store-providerinfo-table"))
	viper.BindPFlag("store.providerweight", serveCmd.Flags().Lookup("store-provider-weight"))
	viper.BindPFlag("store.endpoint", serveCmd.Flags().Lookup("store-endpoint"))
	viper.BindPFlag("delegator.key_file", serveCmd.Flags().Lookup("delegator-key-file"))
	viper.BindPFlag("delegator.indexing_service_web_did", serveCmd.Flags().Lookup("delegator-indexing-service-did"))
	viper.BindPFlag("delegator.indexing_service_proof", serveCmd.Flags().Lookup("delegator-indexing-service-proof"))
	viper.BindPFlag("delegator.upload_service_did", serveCmd.Flags().Lookup("delegator-upload-service-did"))
}
