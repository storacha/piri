package serve

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/storacha/piri/cmd/cli/flags"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/fx/app"
)

var (
	FullCmd = &cobra.Command{
		Use:   "full",
		Short: "Start the full piri server!",
		Args:  cobra.NoArgs,
		RunE:  fullServer,
	}
)

func init() {
	cobra.CheckErr(flags.SetupPDPFlags(FullCmd.Flags()))
	cobra.CheckErr(flags.SetupUCANFlags(FullCmd.Flags()))
}

func fullServer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	userCfg, err := config.Load[config.FullServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

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

		// common dependencies of the PDP and UCAN module:
		//   - identity
		//   - http server
		//   - databases & datastores
		app.CommonModules(appCfg),

		// ucan service dependencies:
		//  - http handlers
		//    - ucan specific handlers, blob allocate and accept, replicate, etc.
		//  - blob, claim, publisher, replicator, and storage services
		app.UCANModule,

		// pdp service dependencies:
		//  - lotus, eth, and contract clients
		//  - piece aggregator
		//  - task and chain scheduler w/ their related tasks
		//  - http handlers
		//    - create proof set, add root, upload piece, etc.
		//  - address wallet
		app.PDPModule,

	)

	// ensure the application was initialized correctly
	if err := fxApp.Err(); err != nil {
		return fmt.Errorf("initalizing piri: %w", err)
	}

	// start the application, triggering lifecycle hooks to start various services and systems
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("starting piri: %w", err)
	}

	go func() {
		// sleep a bit allowing for initial logs to write before printing hello
		time.Sleep(time.Second)
		cliutil.PrintHero(cmd.OutOrStdout(), appCfg.Identity.Signer.DID())
		cmd.Println("Piri Running on: " + appCfg.Server.Host + ":" + strconv.Itoa(int(appCfg.Server.Port)))
		cmd.Println("Piri Public Endpoint: " + appCfg.Server.PublicURL.String())
	}()

	// block: wait for the application to receive a shutdown signal
	<-ctx.Done()
	log.Info("received shutdown signal, beginning graceful shutdown")

	shutdownTimeout := 5 * time.Second
	// Stop the application, with a `shutdownTimeout grace period.
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Info("stopping piri...")
	if err := fxApp.Stop(stopCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorf("graceful shutdown timed out after %s", shutdownTimeout.String())
		}
		return fmt.Errorf("stopping piri: %w", err)
	}
	log.Info("piri stopped successfully")

	// flush any logs before exiting.
	_ = log.Sync()
	return nil

}
