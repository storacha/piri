package setup

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/storacha/go-libstoracha/capabilities/blob"
	"github.com/storacha/go-libstoracha/capabilities/blob/replica"
	"github.com/storacha/go-libstoracha/capabilities/pdp"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/pkg/pdp/types"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/wallet"

	delgclient "github.com/storacha/delegator/client"

	"github.com/storacha/piri/cmd/cli/delegate"
	"github.com/storacha/piri/pkg/config"
	appcfg "github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/fx/app"
	"github.com/storacha/piri/pkg/fx/root"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/presets"
)

var log = logging.Logger("cmd/init")

var InitCmd = &cobra.Command{
	Use:   "init",
	Args:  cobra.NoArgs,
	Short: "Initialize your piri node in the storacha network",
	RunE:  doInit,
}

func init() {
	InitCmd.Flags().String("registrar-url", "https://staging.registrar.warm.storacha.network", "URL of the registrar service")
	cobra.CheckErr(InitCmd.Flags().MarkHidden("registrar-url"))

	InitCmd.Flags().String("data-dir", "", "Path to a data directory Piri will maintain its permanent state in")
	InitCmd.Flags().String("temp-dir", "", "Path to a temporary directory Piri will maintain ephemeral state in")
	InitCmd.Flags().String("key-file", "", "Path to a PEM file containing ed25519 private key used as Piri's identity on the Storacha network")
	InitCmd.Flags().String("wallet-file", "", "Path to a file containing a delegated filecoin address private key in hex format")
	InitCmd.Flags().String("lotus-endpoint", "", "API endpoint of the Lotus node Piri will use to interact with the blockchain")
	InitCmd.Flags().String("operator-email", "", "Email address of the piri operator (your email address for contact with the Storacha team)")
	InitCmd.Flags().String("public-url", "", "URL Piri will advertise to the Storacha network")

	cobra.CheckErr(InitCmd.MarkFlagRequired("data-dir"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("temp-dir"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("key-file"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("wallet-file"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("lotus-endpoint"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("operator-email"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("public-url"))
	InitCmd.SetOut(os.Stdout)
	InitCmd.SetErr(os.Stderr)
}

// initFlags holds all the parsed command flags
type initFlags struct {
	dataDir       string
	tempDir       string
	keyFile       string
	publicURL     *url.URL
	walletPath    string
	lotusEndpoint string
	operatorEmail string
	delegatorURL  string
}

// parseAndValidateFlags parses command flags and validates them
func parseAndValidateFlags(cmd *cobra.Command) (*initFlags, error) {
	dataDir, err := cmd.Flags().GetString("data-dir")
	if err != nil {
		return nil, fmt.Errorf("error reading --data-dir: %w", err)
	}
	tempDir, err := cmd.Flags().GetString("temp-dir")
	if err != nil {
		return nil, fmt.Errorf("error reading --temp-dir: %w", err)
	}
	keyFile, err := cmd.Flags().GetString("key-file")
	if err != nil {
		return nil, fmt.Errorf("error reading --key-file: %w", err)
	}
	publicURL, err := cmd.Flags().GetString("public-url")
	if err != nil {
		return nil, fmt.Errorf("error reading --public-url: %w", err)
	}
	parsedURL, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("parsing --public-url: %w", err)
	}
	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("--public-url must include a scheme (http:// or https://)")
	}

	walletPath, err := cmd.Flags().GetString("wallet-file")
	if err != nil {
		return nil, fmt.Errorf("error reading --wallet-file: %w", err)
	}

	lotusEndpoint, err := cmd.Flags().GetString("lotus-endpoint")
	if err != nil {
		return nil, fmt.Errorf("error reading --lotus-endpoint: %w", err)
	}

	operatorEmail, err := cmd.Flags().GetString("operator-email")
	if err != nil {
		return nil, fmt.Errorf("error reading --operator-email: %w", err)
	}

	delegatorURL, err := cmd.Flags().GetString("registrar-url")
	if err != nil {
		return nil, fmt.Errorf("error reading --registrar-url: %w", err)
	}

	return &initFlags{
		dataDir:       dataDir,
		tempDir:       tempDir,
		keyFile:       keyFile,
		publicURL:     parsedURL,
		walletPath:    walletPath,
		lotusEndpoint: lotusEndpoint,
		operatorEmail: operatorEmail,
		delegatorURL:  delegatorURL,
	}, nil
}

// createNode creates and starts a new Piri node
func createNode(ctx context.Context, flags *initFlags) (*fx.App, *service.PDPService, *appcfg.AppConfig, common.Address, error) {
	walletKey, err := walletKeyFromWalletFile(flags.walletPath)
	if err != nil {
		return nil, nil, nil, common.Address{}, fmt.Errorf("parsing owner address: %w", err)
	}
	cfg := appcfg.AppConfig{
		Identity: lo.Must(config.IdentityConfig{KeyFile: flags.keyFile}.ToAppConfig()),
		Server: appcfg.ServerConfig{
			Host:      "localhost",
			Port:      3000,
			PublicURL: *flags.publicURL,
		},
		Storage: lo.Must(config.RepoConfig{
			DataDir: flags.dataDir,
			TempDir: flags.tempDir,
		}.ToAppConfig()),
		PDPService: lo.Must(config.PDPServiceConfig{
			OwnerAddress:    walletKey.Address.String(),
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   flags.lotusEndpoint,
			SigningServiceConfig: config.SigningServiceConfig{
				Endpoint: presets.SigningServiceEndpoint.String(),
			},
		}.ToAppConfig()),
		Replicator: appcfg.DefaultReplicatorConfig(),
	}

	var (
		pdpSvc *service.PDPService
		wlt    wallet.Wallet
	)
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
		fx.Populate(&pdpSvc, &wlt),
	)

	if err := fxApp.Err(); err != nil {
		return nil, nil, nil, common.Address{}, fmt.Errorf("initializing piri node: %w", err)
	}

	// before we start the service, which on start up checks for a configured wallet, we import the wallet here.
	if _, err := wlt.Import(ctx, &walletKey.KeyInfo); err != nil {
		return nil, nil, nil, common.Address{}, fmt.Errorf("importing wallet: %w", err)
	}

	if err := fxApp.Start(ctx); err != nil {
		return nil, nil, nil, common.Address{}, fmt.Errorf("starting piri node: %w", err)
	}

	return fxApp, pdpSvc, &cfg, walletKey.Address, nil
}

func walletKeyFromWalletFile(walletPath string) (*wallet.Key, error) {
	inpdata, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, fmt.Errorf("reading wallet from file %s: %w", walletPath, err)
	}

	data, err := hex.DecodeString(strings.TrimSpace(string(inpdata)))
	if err != nil {
		return nil, fmt.Errorf("decoding wallet from file %s: %w", walletPath, err)
	}

	var ki struct {
		Type       string
		PrivateKey []byte
	}
	if err := json.Unmarshal(data, &ki); err != nil {
		return nil, err
	}

	return wallet.NewKey(keystore.KeyInfo{PrivateKey: ki.PrivateKey})
}

func registerWithContract(ctx context.Context, id principal.Signer, pdpSvc *service.PDPService) (uint64, error) {
	// check if the provider is already registered with the contract
	status, err := pdpSvc.GetProviderStatus(ctx)
	if err != nil {
		return 0, fmt.Errorf("getting provider status: %w", err)
	}
	// already registered, return the provider id
	if status.IsRegistered {
		return status.ID, nil
	}
	// else we need to register
	res, err := pdpSvc.RegisterProvider(ctx, types.RegisterProviderParams{
		Name:        id.DID().String(),
		Description: "Storacha Service Operator",
	})
	if err != nil {
		return 0, fmt.Errorf("registering provider: %w", err)
	}
	// then wait for transaction to be applied
	if err := pdpSvc.WaitForConfirmation(ctx, res.TransactionHash, (30*time.Second)*4); err != nil {
		return 0, fmt.Errorf("waiting for confirmation of registration: %w", err)
	}
	// so that we may then query for our provider ID
	status, err = pdpSvc.GetProviderStatus(ctx)
	if err != nil {
		return 0, fmt.Errorf("getting provider status: %w", err)
	}
	return status.ID, nil
}

// setupProofSet creates or finds an existing proof set
func setupProofSet(ctx context.Context, cmd *cobra.Command, pdpSvc *service.PDPService, contractAddress common.Address) (uint64, error) {
	proofSets, err := pdpSvc.ListProofSets(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing proof sets: %w", err)
	}

	if len(proofSets) > 1 {
		return 0, fmt.Errorf("multiple proof sets exist, cannot register: %+v", proofSets)
	}

	if len(proofSets) == 1 {
		cmd.PrintErrf("✅ Using existing proof set ID: %d\n", proofSets[0].ID)
		return proofSets[0].ID, nil
	}

	// Create new proof set
	cmd.PrintErrln("📝 Creating new proof set...")
	tx, err := pdpSvc.CreateProofSet(ctx)
	if err != nil {
		return 0, fmt.Errorf("creating proof set: %w", err)
	}

	cmd.PrintErrln("⏳ Waiting for proof set creation to be confirmed on-chain...")
	for {
		time.Sleep(10 * time.Second)
		status, err := pdpSvc.GetProofSetStatus(ctx, tx)
		if err != nil {
			return 0, fmt.Errorf("getting proof set status: %w", err)
		}
		cmd.PrintErrf("   Transaction status: %s\n", status.TxStatus)
		if status.ID > 0 {
			cmd.PrintErrf("✅ Proof set created with ID: %d\n", status.ID)
			return status.ID, nil
		}
	}
}

// registerWithDelegator handles registration with the delegator service
func registerWithDelegator(ctx context.Context, cmd *cobra.Command, cfg *appcfg.AppConfig, flags *initFlags, ownerAddress common.Address, proofSetID uint64) (string, string, error) {
	c, err := delgclient.New(flags.delegatorURL)
	if err != nil {
		return "", "", fmt.Errorf("creating delegator client: %w", err)
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
		return "", "", fmt.Errorf("creating delegation: %w", err)
	}

	nodeProof, err := delegate.FormatDelegation(d.Archive())
	if err != nil {
		return "", "", fmt.Errorf("formatting delegation: %w", err)
	}

	req := &delgclient.RegisterRequest{
		Operator:      cfg.Identity.Signer.DID().String(),
		OwnerAddress:  ownerAddress.String(),
		ProofSetID:    proofSetID,
		OperatorEmail: flags.operatorEmail,
		PublicURL:     flags.publicURL.String(),
		Proof:         nodeProof,
	}

	registered, err := c.IsRegistered(ctx, &delgclient.IsRegisteredRequest{DID: cfg.Identity.Signer.DID().String()})
	if err != nil {
		return "", "", fmt.Errorf("checking registration status: %w", err)
	}

	if !registered {
		err = c.Register(ctx, req)
		if err != nil {
			return "", "", fmt.Errorf("registering with delegator: %w", err)
		}
		cmd.PrintErrln("✅ Successfully registered with delegator service")
	} else {
		cmd.PrintErrln("✅ Node already registered with delegator service")
	}

	// Request proofs from delegator
	cmd.PrintErrln("📥 Requesting proofs from delegator service...")
	res, err := c.RequestProofs(ctx, cfg.Identity.Signer.DID().String())
	if err != nil {
		return "", "", fmt.Errorf("requesting delegator proof: %w", err)
	}

	if res == nil || res.Proofs.Indexer == "" || res.Proofs.EgressTracker == "" {
		return "", "", fmt.Errorf("missing proofs from delegator")
	}

	cmd.PrintErrln("✅ Received proofs from delegator")

	return res.Proofs.Indexer, res.Proofs.EgressTracker, nil
}

func requestContractApproval(ctx context.Context, id principal.Signer, flags *initFlags, ownerAddress common.Address) error {
	// create a signature by signing our own did with the private key of our did
	signature := id.Sign(id.DID().Bytes()).Raw()

	c, err := delgclient.New(flags.delegatorURL)
	if err != nil {
		return fmt.Errorf("creating delegator client: %w", err)
	}

	// requesting approval requires the message to be published to chain by delegator
	// before it returns, so we need an extended timeout
	// TODO a better(?) mechanism might be to poll via a different method
	c = c.WithHTTPClient(&http.Client{
		Timeout: 5 * time.Minute,
	})

	req := &delgclient.RequestApprovalRequest{
		Operator:     id.DID().String(),
		OwnerAddress: ownerAddress.String(),
		Signature:    signature,
	}

	// request approval from delegator, on success the delegator will approve piri within the smart contract
	return c.RequestApproval(ctx, req)
}

// generateConfig generates the final configuration for the user
func generateConfig(cfg *appcfg.AppConfig, flags *initFlags, ownerAddress common.Address, proofSetID uint64, indexerProof string, egressTrackerProof string) (config.FullServerConfig, error) {
	return config.FullServerConfig{
		Identity: config.IdentityConfig{KeyFile: flags.keyFile},
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
			OwnerAddress:    ownerAddress.String(),
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   flags.lotusEndpoint,
		},
		UCANService: config.UCANServiceConfig{
			Services: config.ServicesConfig{
				Indexer: config.IndexingServiceConfig{
					Proof: indexerProof,
				},
				EgressTracker: config.EgressTrackerServiceConfig{
					Proof:             egressTrackerProof,
					MaxBatchSizeBytes: 10 * 1024,
				},
			},
			ProofSetID: proofSetID,
		},
	}, nil
}

func doInit(cmd *cobra.Command, _ []string) error {
	logging.SetAllLoggers(logging.LevelFatal)
	ctx := context.Background()

	cmd.PrintErrln("🚀 Initializing your Piri node on the Storacha Network...")
	cmd.PrintErrln()

	// Step 1: Parse and validate flags
	cmd.PrintErrln("[1/7] Validating configuration...")
	flags, err := parseAndValidateFlags(cmd)
	if err != nil {
		return err
	}
	cmd.PrintErrln("✅ Configuration validated")
	cmd.PrintErrln()

	// Step 2: Create and start node
	cmd.PrintErrln("[2/7] Creating Piri node...")
	fxApp, pdpSvc, cfg, ownerAddress, err := createNode(ctx, flags)
	if err != nil {
		return err
	}
	defer fxApp.Stop(ctx)
	cmd.PrintErrf("✅ Node created with DID: %s\n", cfg.Identity.Signer.DID().String())
	cmd.PrintErrln()

	// Step 3: Register with the smart contract
	cmd.PrintErrln("[3/7] Registering provider with contract...")
	providerID, err := registerWithContract(ctx, cfg.Identity.Signer, pdpSvc)
	if err != nil {
		return err
	}
	cmd.PrintErrf("✅ Node registered with contract ProviderID: %d\n", providerID)
	cmd.PrintErrln()

	// Step 4: Request approval to join contract from storacha
	cmd.PrintErrln("[4/7] Requesting approval to join contract from Storacha...")
	if err := requestContractApproval(ctx, cfg.Identity.Signer, flags, ownerAddress); err != nil {
		return err
	}
	cmd.PrintErrln("✅ Node approved to join contract by Storacha")
	cmd.PrintErrln()

	// Step 5: Create or find proof set (must be approved in step 4 to succeed here)
	cmd.PrintErrln("[5/7] Setting up proof set...")
	proofSetID, err := setupProofSet(ctx, cmd, pdpSvc, cfg.PDPService.ContractAddress)
	if err != nil {
		return err
	}
	cmd.PrintErrln()

	// Step 6: Register with delegator service
	cmd.PrintErrln("[6/7] Registering with delegator service...")
	indexerProof, egressTrackerProof, err := registerWithDelegator(ctx, cmd, cfg, flags, ownerAddress, proofSetID)
	if err != nil {
		return err
	}
	cmd.PrintErrln()

	// Step 7: Generate configuration
	cmd.PrintErrln("[7/7] Generating configuration file...")
	userConfig, err := generateConfig(cfg, flags, ownerAddress, proofSetID, indexerProof, egressTrackerProof)
	if err != nil {
		return err
	}

	cfgData, err := toml.Marshal(userConfig)
	if err != nil {
		return fmt.Errorf("marshaling configuration: %w", err)
	}

	cmd.PrintErrln("\n🎉 Initialization complete! Your configuration:")

	// Write to both stdout and file using TeeWriter
	configFile, err := os.Create(PiriConfigFileName)
	if err != nil {
		// If we can't create the file, just write to stdout
		cmd.PrintErrf("Warning: Failed to create %s: %v\n", PiriConfigFileName, err)
		cmd.Print(string(cfgData))
		return nil
	}
	defer configFile.Close()

	// Use TeeWriter to write to both stdout and file
	teeWriter := io.MultiWriter(cmd.OutOrStdout(), configFile)
	if _, err := teeWriter.Write(cfgData); err != nil {
		cmd.PrintErrf("Error writing configuration: %v\n", err)
	}

	cmd.PrintErrf("\nConfiguration saved to: %s\n", PiriConfigFileName)
	return nil
}
