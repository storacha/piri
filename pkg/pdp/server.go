package pdp

import (
	"context"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/hashicorp/go-multierror"

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
	return nil, nil
	/*
			ds, err := leveldb.NewDatastore(filepath.Join(dataDir, "datastore"), nil)
			if err != nil {
				return nil, err
			}
			blobStore := blobstore.NewTODO_DsBlobstore(namespace.Wrap(ds, datastore.NewKey("blobs")))
			stashStore, err := store.NewStashStore(path.Join(dataDir))
			if err != nil {
				return nil, err
			}
			if has, err := wlt.Has(ctx, address); err != nil {
				return nil, fmt.Errorf("failed to read wallet for address %s: %w", address, err)
			} else if !has {
				return nil, fmt.Errorf("wallet for address %s not found", address)
			}
			lotusURL, err := url.Parse(lotusUrl)
			if err != nil {
				return nil, fmt.Errorf("parsing lotus client address: %w", err)
			}
			if lotusURL.Scheme != "ws" && lotusURL.Scheme != "wss" {
				return nil, fmt.Errorf("lotus client address must be 'ws' or 'wss', got %s", lotusURL.Scheme)
			}
			chainClient, chainClientCloser, err := client.NewFullNodeRPCV1(ctx, lotusURL.String(), nil)
			if err != nil {
				return nil, err
			}
	>>>>>>> 354c391 (massive wip need to break this up. but single process mode works!)

			ethClient, err := ethclient.Dial(lotusUrl)
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
				// wait up to 5 seconds before failing to write due to busted database.
				database.WithTimeout(5*time.Second))

			if err != nil {
				return nil, err
			}
			//pdpService, err := service.NewPDPService(stateDB, address, wlt, blobStore, stashStore, chainClient, ethClient, &contract.PDPContract{})
			if err != nil {
				return nil, fmt.Errorf("creating pdp service: %w", err)
			}
			pdpAPI := &server.PDP{Service: pdpService}
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
	*/
}
