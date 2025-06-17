package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/wallet"
)

var ServeCmd = &cli.Command{
	Name: "serve",
	Subcommands: []*cli.Command{
		pdpCmd,
	},
}

var pdpCmd = &cli.Command{
	Name:  "pdp",
	Usage: "TODO",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   3001,
			Usage:   "Port to bind the server to.",
			EnvVars: []string{"PDP_SERVER_PORT"},
			Action: func(c *cli.Context, v int) error {
				if v <= 0 || v > 65535 {
					return fmt.Errorf("invalid port: must be between 1 and 65535")
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:    "data-dir",
			Aliases: []string{"d"},
			Usage:   "Root directory to store data in.",
			EnvVars: []string{"PIRI_PDP_DATA_DIR"},
		},
		&cli.StringFlag{
			Name:    "tmp-dir",
			Aliases: []string{"t"},
			Usage:   "Temporary directory data is uploaded to before being moved to data-dir.",
			EnvVars: []string{"PIRI_PDP_TMP_DIR"},
		},
		&cli.StringFlag{
			Name:     "lotus-host-url",
			Aliases:  []string{"lotus-client-host", "eth-client-host"},
			Usage:    "URL of a Lotus node that provides both Lotus and Ethereum API endpoints",
			Value:    "ws://127.0.0.1:1234/rpc/v1",
			EnvVars:  []string{"PIRI_LOTUS_HOST_URL", "LOTUS_CLIENT_HOST", "ETH_CLIENT_HOST"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pdp-address",
			Usage:    "A hex encoded delegate address for interacting with PDP contact",
			Required: true,
			Action: func(context *cli.Context, s string) error {
				if !common.IsHexAddress(s) {
					return fmt.Errorf("invalid address %s", s)
				}
				return nil
			},
		},
	},
	Action: func(cctx *cli.Context) error {
		logging.SetLogLevel("*", "INFO")
		rootDir := cctx.String("data-dir")
		if rootDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting user home directory: %w", err)
			}

			dir, err := mkdirp(homeDir, ".storacha")
			if err != nil {
				return err
			}
			log.Warnf("Data directory is not configured, using default: %s", dir)
			rootDir = dir
		}

		ctx := cctx.Context
		port := cctx.Int("port")
		lotusURL := cctx.String("lotus-host-url")
		ethURL := cctx.String("lotus-host-url")
		addrStr := cctx.String("pdp-address")

		walletDir, err := mkdirp(rootDir, "wallet")
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

		dataDir, err := mkdirp(rootDir, "pdp")
		if err != nil {
			return err
		}

		svr, err := pdp.NewServer(
			ctx,
			dataDir,
			port,
			lotusURL,
			ethURL,
			common.HexToAddress(addrStr),
			wlt,
		)
		if err != nil {
			return fmt.Errorf("creating pdp server: %w", err)
		}

		if err := svr.Start(ctx); err != nil {
			return fmt.Errorf("starting pdp server: %w", err)
		}
		fmt.Println("Server started! Listening on ", port)

		<-ctx.Done()

		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := svr.Stop(stopCtx); err != nil {
			return fmt.Errorf("stopping pdp server: %w", err)
		}
		return nil
	},
}
