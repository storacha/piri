package pdp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	leveldb "github.com/ipfs/go-ds-leveldb"

	"github.com/storacha/piri/pkg/database"
	"github.com/storacha/piri/pkg/database/gormdb"
	"github.com/storacha/piri/pkg/pdp/httpapi/server"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
	"github.com/storacha/piri/pkg/wallet"
)

type Server struct {
	startFuncs []func(ctx context.Context) error
	stopFuncs  []func(ctx context.Context) error
}

func (s *Server) Start(ctx context.Context) error {
	for _, startFunc := range s.startFuncs {
		if err := startFunc(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	var errs error
	for _, stopFunc := range s.stopFuncs {
		if err := stopFunc(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func NewServer(
	ctx context.Context,
	dataDir string,
	endpoint *url.URL,
	lotusEndpoint *url.URL,
	address common.Address,
	wlt *wallet.LocalWallet,
) (*Server, error) {
	ds, err := leveldb.NewDatastore(filepath.Join(dataDir, "datastore"), nil)
	if err != nil {
		return nil, err
	}
	blobStore := blobstore.NewTODO_DsBlobstore(namespace.Wrap(ds, datastore.NewKey("blobs")))
	stashStore, err := stashstore.NewStashStore(path.Join(dataDir))
	if err != nil {
		return nil, err
	}
	if has, err := wlt.Has(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to read wallet for address %s: %w", address, err)
	} else if !has {
		return nil, fmt.Errorf("wallet for address %s not found", address)
	}
	chainClient, chainClientCloser, err := client.NewFullNodeRPCV1(ctx, lotusEndpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	ethClient, err := ethclient.Dial(lotusEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("connecting to eth client: %w", err)
	}

	stateDir := filepath.Join(dataDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, err
	}

	stateDB, err := gormdb.New(filepath.Join(stateDir, "state.db"),
		// use a write ahead log for transactions, good for parallel operations.
		database.WithJournalMode(database.JournalModeWAL),
		// ensure foreign key constraints are respected.
		database.WithForeignKeyConstraintsEnable(true),
		// wait up to 5 seconds before failing to write due to bust database.
		database.WithTimeout(5*time.Second))

	if err != nil {
		return nil, err
	}
	pdpService, err := service.SetupPDPService(stateDB, address, wlt, blobStore, stashStore, chainClient, ethClient, &contract.PDPContract{})
	if err != nil {
		return nil, fmt.Errorf("creating pdp service: %w", err)
	}

	pdpAPI := &server.PDPHandler{Service: pdpService}
	svr := server.NewServer(pdpAPI)
	return &Server{
		startFuncs: []func(ctx context.Context) error{
			func(ctx context.Context) error {
				if err := svr.Start(fmt.Sprintf(":%s", endpoint.Port())); err != nil {
					return fmt.Errorf("starting local pdp server: %w", err)
				}
				if err := pdpService.Start(ctx); err != nil {
					return fmt.Errorf("starting pdp service: %w", err)
				}
				return nil
			},
		},
		stopFuncs: []func(context.Context) error{
			func(ctx context.Context) error {
				var errs error
				if err := svr.Shutdown(ctx); err != nil {
					errs = multierror.Append(errs, err)
				}
				if err := pdpService.Stop(ctx); err != nil {
					errs = multierror.Append(errs, err)
				}
				chainClientCloser()
				ethClient.Close()
				return errs
			},
		},
	}, nil

}
