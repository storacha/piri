package initalize

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/delegation"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/cmd/cli/delegate"
	"github.com/storacha/piri/pkg/config"
	appcfg "github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/fx/root"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/registration/delgclient"
)

var log = logging.Logger("cmd/init")

var InitCmd = &cobra.Command{
	Use:   "init",
	Args:  cobra.NoArgs,
	Short: "Initialize your piri node in the storacha network",
	RunE:  doInit,
}

func init() {
	// key file comes from the root command

	// TODO make hidden
	InitCmd.Flags().String("delegator-url", "http://localhost:8080", "URL of the delegator service")

	InitCmd.Flags().String("lotus-endpoint", "", "API endpoint of your lotus node")
	// TODO consider making this a path to the private key file, then handle importing
	InitCmd.Flags().String("owner-address", "", "Ethereum address of the owner")
	InitCmd.Flags().String("operator-email", "", "Email address of the operator")
	InitCmd.Flags().String("public-url", "", "Public URL of the operator's service")

	cobra.CheckErr(InitCmd.MarkFlagRequired("owner-address"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("lotus-endpoint"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("operator-email"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("public-url"))
}

// initFlags holds all the parsed command flags
type initFlags struct {
	publicURL     *url.URL
	ownerAddress  string
	lotusEndpoint string
	operatorEmail string
	delegatorURL  string
}

// parseAndValidateFlags parses command flags and validates them
func parseAndValidateFlags(cmd *cobra.Command) (*initFlags, error) {
	publicURL, err := cmd.Flags().GetString("public-url")
	if err != nil {
		return nil, fmt.Errorf("getting public-url flag: %w", err)
	}
	parsedURL, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("parsing public-url: %w", err)
	}
	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("public-url must include a scheme (http:// or https://)")
	}

	ownerAddress, err := cmd.Flags().GetString("owner-address")
	if err != nil {
		return nil, fmt.Errorf("getting owner-address flag: %w", err)
	}
	if !common.IsHexAddress(ownerAddress) {
		return nil, fmt.Errorf("owner-address is not a valid Ethereum address")
	}

	lotusEndpoint, err := cmd.Flags().GetString("lotus-endpoint")
	if err != nil {
		return nil, fmt.Errorf("getting lotus-endpoint flag: %w", err)
	}

	operatorEmail, err := cmd.Flags().GetString("operator-email")
	if err != nil {
		return nil, fmt.Errorf("getting operator-email flag: %w", err)
	}

	delegatorURL, err := cmd.Flags().GetString("delegator-url")
	if err != nil {
		return nil, fmt.Errorf("getting delegator-url flag: %w", err)
	}

	return &initFlags{
		publicURL:     parsedURL,
		ownerAddress:  ownerAddress,
		lotusEndpoint: lotusEndpoint,
		operatorEmail: operatorEmail,
		delegatorURL:  delegatorURL,
	}, nil
}

// createNode creates and starts a new Piri node
func createNode(ctx context.Context, flags *initFlags) (*fx.App, *service.PDPService, *appcfg.AppConfig, error) {
	cfg := appcfg.AppConfig{
		Identity: lo.Must(config.IdentityConfig{KeyFile: viper.GetString("identity.key_file")}.ToAppConfig()),
		Server: appcfg.ServerConfig{
			Host:      "localhost",
			Port:      3000,
			PublicURL: *flags.publicURL,
		},
		Storage: lo.Must(config.RepoConfig{
			DataDir: viper.GetString("repo.data_dir"),
			TempDir: viper.GetString("repo.temp_dir"),
		}.ToAppConfig()),
		PDPService: lo.Must(config.PDPServiceConfig{
			OwnerAddress:    flags.ownerAddress,
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   flags.lotusEndpoint,
		}.ToAppConfig()),
	}

	var pdpSvc *service.PDPService
	fxApp := fx.New(
		fx.RecoverFromPanics(),
		fx.WithLogger(func() fxevent.Logger {
			el := &fxevent.ZapLogger{Logger: log.Desugar()}
			el.UseLogLevel(zapcore.DebugLevel)
			return el
		}),
		app.CommonModules(cfg),
		app.PDPModule,
		root.Module,
		fx.Populate(&pdpSvc),
	)

	if err := fxApp.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("initializing piri node: %w", err)
	}

	if err := fxApp.Start(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("starting piri node: %w", err)
	}

	return fxApp, pdpSvc, &cfg, nil
}

// setupProofSet creates or finds an existing proof set
func setupProofSet(ctx context.Context, cmd *cobra.Command, pdpSvc *service.PDPService, contractAddress common.Address) (uint64, error) {
	proofSets, err := pdpSvc.ListProofSets(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing proof sets: %w", err)
	}

	if len(proofSets) > 1 {
		return 0, fmt.Errorf("multiple proof sets exist, cannot register: %v", proofSets)
	}

	if len(proofSets) == 1 {
		cmd.Printf("✅ Using existing proof set ID: %d\n", proofSets[0].ID)
		return proofSets[0].ID, nil
	}

	// Create new proof set
	cmd.Println("📝 Creating new proof set...")
	tx, err := pdpSvc.CreateProofSet(ctx, contractAddress)
	if err != nil {
		return 0, fmt.Errorf("creating proof set: %w", err)
	}

	cmd.Println("⏳ Waiting for proof set creation to be confirmed on-chain...")
	for {
		time.Sleep(5 * time.Second)
		status, err := pdpSvc.GetProofSetStatus(ctx, tx)
		if err != nil {
			return 0, fmt.Errorf("getting proof set status: %w", err)
		}
		cmd.Printf("   Transaction status: %s\n", status.TxStatus)
		if status.ID > 0 {
			cmd.Printf("✅ Proof set created with ID: %d\n", status.ID)
			return status.ID, nil
		}
	}
}

// registerWithDelegator handles registration with the delegator service
func registerWithDelegator(ctx context.Context, cmd *cobra.Command, cfg *appcfg.AppConfig, flags *initFlags, proofSetID uint64) (string, error) {
	c, err := delgclient.New(flags.delegatorURL)
	if err != nil {
		return "", fmt.Errorf("creating delegator client: %w", err)
	}

	// Generate delegation proof for upload service
	d, err := delegate.MakeDelegation(
		cfg.Identity.Signer,
		presets.UploadServiceDID,
		[]string{
			blob.AllocateAbility,
			blob.AcceptAbility,
			pdp.InfoAbility,
			replica.AllocateAbility,
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return "", fmt.Errorf("creating delegation: %w", err)
	}

	nodeProof, err := delegate.FormatDelegation(d.Archive())
	if err != nil {
		return "", fmt.Errorf("formatting delegation: %w", err)
	}

	req := &delgclient.RegisterRequest{
		DID:           cfg.Identity.Signer.DID().String(),
		OwnerAddress:  flags.ownerAddress,
		ProofSetID:    proofSetID,
		OperatorEmail: flags.operatorEmail,
		PublicURL:     flags.publicURL.String(),
		Proof:         nodeProof,
	}

	registered, err := c.IsRegistered(ctx, &delgclient.IsRegisteredRequest{DID: cfg.Identity.Signer.DID().String()})
	if err != nil {
		return "", fmt.Errorf("checking registration status: %w", err)
	}

	if !registered {
		err = c.Register(ctx, req)
		if err != nil {
			return "", fmt.Errorf("registering with delegator: %w", err)
		}
		cmd.Println("✅ Successfully registered with delegator service")
	} else {
		cmd.Println("✅ Node already registered with delegator service")
	}

	// Request proof from delegator
	cmd.Println("📥 Requesting proof from delegator service...")
	delegatorProof, err := c.RequestProof(ctx, cfg.Identity.Signer.DID().String())
	if err != nil {
		return "", fmt.Errorf("requesting delegator proof: %w", err)
	}
	cmd.Println("✅ Received delegator proof")

	return delegatorProof.Proof, nil
}

// generateConfig generates the final configuration for the user
func generateConfig(cfg *appcfg.AppConfig, flags *initFlags, proofSetID uint64, delegatorProof string) (config.FullServerConfig, error) {
	return config.FullServerConfig{
		Identity: config.IdentityConfig{KeyFile: viper.GetString("identity.key_file")},
		Repo: config.RepoConfig{
			DataDir: cfg.Storage.DataDir,
			TempDir: cfg.Storage.TempDir,
		},
		Server: config.ServerConfig{
			Port:      cfg.Server.Port,
			Host:      cfg.Server.Host,
			PublicURL: flags.publicURL.String(),
		},
		PDPService: config.PDPServiceConfig{
			OwnerAddress:    flags.ownerAddress,
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   flags.lotusEndpoint,
		},
		UCANService: config.UCANServiceConfig{
			Services: config.ServicesConfig{
				Indexer: config.IndexingServiceConfig{
					Proof: delegatorProof,
				},
			},
			ProofSetID: proofSetID,
		},
	}, nil
}

func doInit(cmd *cobra.Command, _ []string) error {
	logging.SetAllLoggers(logging.LevelError)
	ctx := context.Background()

	cmd.Println("🚀 Initializing your Piri node in the Storacha network...")
	cmd.Println()

	// Step 1: Parse and validate flags
	cmd.Println("[1/5] Validating configuration...")
	flags, err := parseAndValidateFlags(cmd)
	if err != nil {
		return err
	}
	cmd.Println("✅ Configuration validated")
	cmd.Println()

	// Step 2: Create and start node
	cmd.Println("[2/5] Creating Piri node...")
	fxApp, pdpSvc, cfg, err := createNode(ctx, flags)
	if err != nil {
		return err
	}
	defer fxApp.Stop(ctx)
	cmd.Printf("✅ Node created with DID: %s\n", cfg.Identity.Signer.DID().String())
	cmd.Println()

	// Step 3: Create or find proof set
	cmd.Println("[3/5] Setting up proof set...")
	proofSetID, err := setupProofSet(ctx, cmd, pdpSvc, cfg.PDPService.ContractAddress)
	if err != nil {
		return err
	}
	cmd.Println()

	// Step 4: Register with delegator service
	cmd.Println("[4/5] Registering with delegator service...")
	delegatorProof, err := registerWithDelegator(ctx, cmd, cfg, flags, proofSetID)
	if err != nil {
		return err
	}
	cmd.Println()

	// Step 5: Generate configuration
	cmd.Println("[5/5] Generating configuration file...")
	userConfig, err := generateConfig(cfg, flags, proofSetID, delegatorProof)
	if err != nil {
		return err
	}

	cfgData, err := toml.Marshal(userConfig)
	if err != nil {
		return fmt.Errorf("marshaling configuration: %w", err)
	}

	cmd.Println("\n🎉 Initialization complete! Your configuration:")
	cmd.Println("\n" + string(cfgData))

	return nil
}
