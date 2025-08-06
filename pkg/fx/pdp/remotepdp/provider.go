package remotepdp

import (
	"fmt"

	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var Module = fx.Module("remote-pdp",
	fx.Provide(
		ProvidePDP,
	),
)

// ProvidePDP provides a PDP implementation based on configuration
func ProvidePDP(cfg app.AppConfig, id principal.Signer, receiptStore receiptstore.ReceiptStore) (pdp.PDP, error) {
	// If no PDP server is configured, return nil
	if cfg.Services.PDPServer == nil {
		return nil, nil
	}

	pdpCfg := cfg.Services.PDPServer

	// Validate configuration
	if pdpCfg.ProofSet == 0 {
		return nil, fmt.Errorf("must set proof-set when using pdp-server-url")
	}

	// Create aggregator datastore
	aggDs, err := leveldb.NewDatastore(cfg.Storage.Aggregator.DatastoreDir, nil)
	if err != nil {
		return nil, fmt.Errorf("creating aggregator datastore: %w", err)
	}

	// Create remote PDP service
	pdpService, err := pdp.NewRemote(
		&pdp.Config{
			PDPDatastore: aggDs,
			PDPServerURL: pdpCfg.URL,
			ProofSet:     pdpCfg.ProofSet,
			DatabasePath: cfg.Storage.Replicator.DBPath,
		},
		id,
		receiptStore,
	)
	if err != nil {
		return nil, fmt.Errorf("creating remote PDP service: %w", err)
	}

	return pdpService, nil
}
