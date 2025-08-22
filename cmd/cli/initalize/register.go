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
	// TODO consider making this a path to the pirvate key file, then handle importing
	InitCmd.Flags().String("owner-address", "", "Ethereum address of the owner")
	InitCmd.Flags().String("operator-email", "", "Email address of the operator")
	InitCmd.Flags().String("public-url", "", "Public URL of the operator's service")

	cobra.CheckErr(InitCmd.MarkFlagRequired("owner-address"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("lotus-endpoint"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("operator-email"))
	cobra.CheckErr(InitCmd.MarkFlagRequired("public-url"))
}

func doInit(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	publicURL, err := cmd.Flags().GetString("public-url")
	if err != nil {
		return fmt.Errorf("getting public-url flag: %w", err)
	}
	parsedURL, err := url.Parse(publicURL)
	if err != nil {
		return fmt.Errorf("parsing public-url: %w", err)
	}
	// TODO assert the URL has a schema

	ownerAddress, err := cmd.Flags().GetString("owner-address")
	if err != nil {
		return fmt.Errorf("getting owner-address flag: %w", err)
	}
	if !common.IsHexAddress(ownerAddress) {
		return fmt.Errorf("owner-address is not a valid address")
	}

	lotusEndpoint, err := cmd.Flags().GetString("lotus-endpoint")
	if err != nil {
		return fmt.Errorf("getting delegator-url flag: %w", err)
	}

	cfg := appcfg.AppConfig{
		Identity: lo.Must(config.IdentityConfig{KeyFile: viper.GetString("identity.key_file")}.ToAppConfig()),
		Server: appcfg.ServerConfig{
			Host:      "localhost",
			Port:      3000,
			PublicURL: *parsedURL,
		},
		Storage: lo.Must(config.RepoConfig{
			DataDir: viper.GetString("repo.data_dir"),
			TempDir: viper.GetString("repo.temp_dir"),
		}.ToAppConfig()),
		PDPService: lo.Must(config.PDPServiceConfig{
			OwnerAddress:    ownerAddress,
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   lotusEndpoint,
		}.ToAppConfig()),
	}

	var pdpSvc *service.PDPService
	fxApp := fx.New(
		// if a panic occurs during operation, recover from it and exit (somewhat) gracefully.
		fx.RecoverFromPanics(),
		// provide fx with our logger for its events logged at debug level.
		// any fx errors will still be logged at the error level.
		fx.WithLogger(func() fxevent.Logger {
			el := &fxevent.ZapLogger{Logger: log.Desugar()}
			el.UseLogLevel(zapcore.DebugLevel)
			return el
		}),
		app.CommonModules(cfg),
		app.PDPModule,
		// need this api, might be better in common
		root.Module,

		// pull out the pdp service we need to make a proof set
		fx.Populate(&pdpSvc),
	)

	// ensure the application was initialized correctly
	if err := fxApp.Err(); err != nil {
		return fmt.Errorf("initalizing piri: %w", err)
	}

	// start the application, triggering lifecycle hooks to start various services and systems
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting piri: %w", err)
	}

	proofSets, err := pdpSvc.ListProofSets(ctx)
	if err != nil {
		return fmt.Errorf("listing proof sets: %w", err)
	}
	// TODO consider relaxing this, in the event creating a proofset works, but registration fails
	if len(proofSets) > 1 {
		return fmt.Errorf("multipule proof sets exist, cannot registet: %v", proofSets)
	}
	proofSetID := uint64(0)
	if len(proofSets) == 1 {
		proofSetID = proofSets[0].ID
	} else {
		// now we are pretty sure there isn't a proof set, let's make one
		tx, err := pdpSvc.CreateProofSet(ctx, cfg.PDPService.ContractAddress)
		if err != nil {
			return fmt.Errorf("creating proof set: %w", err)
		}

		// wait for it to be created
		done := false
		for !done {
			time.Sleep(5 * time.Second)
			status, err := pdpSvc.GetProofSetStatus(ctx, tx)
			if err != nil {
				return fmt.Errorf("getting proof set status: %w", err)
			}
			cmd.Println("Proof Set Status: " + status.TxStatus)
			if status.ID > 0 {
				done = true
				proofSetID = status.ID
			}
		}
	}
	delegatorURL, err := cmd.Flags().GetString("delegator-url")
	if err != nil {
		return fmt.Errorf("getting delegator-url flag: %w", err)
	}

	c, err := delgclient.New(delegatorURL)
	if err != nil {
		return fmt.Errorf("creating delegator client: %w", err)
	}

	operatorEmail, err := cmd.Flags().GetString("operator-email")
	if err != nil {
		return fmt.Errorf("getting operator-email flag: %w", err)
	}

	// generate a proof for upload service

	d, err := delegate.MakeDelegation(
		cfg.Identity.Signer,      // issuer is this node operator
		presets.UploadServiceDID, // audience is the upload service
		[]string{
			blob.AllocateAbility,
			blob.AcceptAbility,
			pdp.InfoAbility,
			replica.AllocateAbility,
		},
		delegation.WithNoExpiration(),
	)
	if err != nil {
		return fmt.Errorf("creating delegation: %w", err)
	}

	nodeProof, err := delegate.FormatDelegation(d.Archive())
	if err != nil {
		return fmt.Errorf("formatting delegation as multibase-base64-encoded CIDv1: %w", err)
	}
	req := &delgclient.RegisterRequest{
		DID:           cfg.Identity.Signer.DID().String(),
		OwnerAddress:  ownerAddress,
		ProofSetID:    proofSetID,
		OperatorEmail: operatorEmail,
		PublicURL:     parsedURL.String(),
		Proof:         nodeProof,
	}

	registered, err := c.IsRegistered(ctx, &delgclient.IsRegisteredRequest{DID: cfg.Identity.Signer.DID().String()})
	if err != nil {
		return fmt.Errorf("isRegistered: %w", err)
	}
	if !registered {
		err = c.Register(ctx, req)
		if err != nil {
			return fmt.Errorf("registering with delegator: %w", err)
		}
		cmd.Println("Successfully registered with delegator service")
	}

	delegatorProof, err := c.RequestProof(ctx, cfg.Identity.Signer.DID().String())
	if err != nil {
		return fmt.Errorf("getting delegator proof set: %w", err)
	}

	cmd.Println(delegatorProof.Proof)

	// we have created our proofset and registered with the delegator, time to make a config for the user
	userConfig := config.FullServerConfig{
		Identity: config.IdentityConfig{KeyFile: viper.GetString("identity.key_file")},
		Repo: config.RepoConfig{
			DataDir: cfg.Storage.DataDir,
			TempDir: cfg.Storage.TempDir,
		},
		Server: config.ServerConfig{
			Port:      cfg.Server.Port,
			Host:      cfg.Server.Host,
			PublicURL: parsedURL.String(),
		},
		PDPService: config.PDPServiceConfig{
			OwnerAddress:    ownerAddress,
			ContractAddress: presets.PDPRecordKeeperAddress,
			LotusEndpoint:   lotusEndpoint,
		},
		UCANService: config.UCANServiceConfig{
			Services: config.ServicesConfig{
				Indexer: config.IndexingServiceConfig{
					Proof: delegatorProof.Proof,
				},
			},
			ProofSetID: proofSetID,
		},
	}
	cfgData, err := toml.Marshal(userConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal user config: %w", err)
	}
	cmd.Println(string(cfgData))
	// we can kill the server safely now, maybe could do this earlier even.
	return fxApp.Stop(ctx)
}
