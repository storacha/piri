package leveldb

import (
	"fmt"

	leveldb "github.com/ipfs/go-ds-leveldb"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
)

const AllocationStoreDir = "allocation"

func NewAllocationStore(cfg app.Config) (allocationstore.AllocationStore, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("no data dir provided for allocation store")
	}

	p, err := Mkdirp(cfg.DataDir, AllocationStoreDir)
	if err != nil {
		return nil, fmt.Errorf("creating allocation store: %w", err)
	}

	ds, err := leveldb.NewDatastore(p, nil)
	if err != nil {
		return nil, fmt.Errorf("creating leveldb for allocation store: %w", err)
	}

	return allocationstore.NewDsAllocationStore(ds)
}

const BlobStoreDir = "blobs"

func NewBlobStore(cfg app.Config) (blobstore.Blobstore, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("no data dir provided for blob store")
	}

	p, err := Mkdirp(cfg.DataDir, BlobStoreDir)
	if err != nil {
		return nil, fmt.Errorf("creating blob store: %w", err)
	}

	ds, err := leveldb.NewDatastore(p, nil)
	if err != nil {
		return nil, fmt.Errorf("creating leveldb for blob store: %w", err)
	}

	return blobstore.NewDsBlobstore(ds), nil
}

const ClaimStoreDir = "claims"

func NewClaimStore(cfg app.Config) (claimstore.ClaimStore, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("no data dir provided for claim store")
	}

	p, err := Mkdirp(cfg.DataDir, ClaimStoreDir)
	if err != nil {
		return nil, fmt.Errorf("creating claim store: %w", err)
	}

	ds, err := leveldb.NewDatastore(p, nil)
	if err != nil {
		return nil, fmt.Errorf("creating leveldb for claim store: %w", err)
	}

	return delegationstore.NewDsDelegationStore(ds)
}

const PublisherStoreDir = "publisher"

func NewPublisherStore(cfg app.Config) publish
