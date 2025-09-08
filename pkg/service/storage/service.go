package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"runtime"

	"github.com/ipfs/go-datastore"
	"github.com/ipni/go-libipni/maurl"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/pkg/database/sqlitedb"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue/serializer"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/service/claims"
	"github.com/storacha/piri/pkg/service/replicator"
	replicahandler "github.com/storacha/piri/pkg/service/storage/handlers/replica"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

type StorageService struct {
	id            principal.Signer
	blobs         blobs.Blobs
	claims        claims.Claims
	pdp           pdp.PDP
	receiptStore  receiptstore.ReceiptStore
	replicator    replicator.Replicator
	uploadService client.Connection
	startFuncs    []func(ctx context.Context) error
	closeFuncs    []func(ctx context.Context) error
	io.Closer
}

func (s *StorageService) Replicator() replicator.Replicator {
	return s.replicator
}

func (s *StorageService) UploadConnection() client.Connection {
	return s.uploadService
}

func (s *StorageService) Blobs() blobs.Blobs {
	return s.blobs
}

func (s *StorageService) Claims() claims.Claims {
	return s.claims
}

func (s *StorageService) ID() principal.Signer {
	return s.id
}

func (s *StorageService) PDP() pdp.PDP {
	return s.pdp
}

func (s *StorageService) Receipts() receiptstore.ReceiptStore {
	return s.receiptStore
}

func (s *StorageService) Startup(ctx context.Context) error {
	var err error
	for _, startFunc := range s.startFuncs {
		err = errors.Join(startFunc(ctx))
	}
	s.startFuncs = []func(ctx context.Context) error{}
	return err
}

func (s *StorageService) Close(ctx context.Context) error {
	var err error
	for _, close := range s.closeFuncs {
		err = errors.Join(close(ctx))
	}
	s.closeFuncs = []func(context.Context) error{}
	return err
}

var _ Service = (*StorageService)(nil)

func New(opts ...Option) (*StorageService, error) {
	c := &config{}
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	id := c.id
	if id == nil {
		log.Warn("Generating a server identity as one has not been configured!")
		signer, err := ed25519.Generate()
		if err != nil {
			return nil, err
		}
		id = signer
	}
	log.Infof("Server ID: %s", id.DID())

	var closeFuncs []func(context.Context) error
	var startFuncs []func(ctx context.Context) error

	blobOpts := []blobs.Option{}

	if c.allocationStore == nil {
		allocDs := c.allocationDatastore
		if allocDs == nil {
			allocDs = datastore.NewMapDatastore()
			log.Warn("Allocation datastore not configured, using in-memory datastore")
		}
		closeFuncs = append(closeFuncs, func(context.Context) error { return allocDs.Close() })
		blobOpts = append(blobOpts, blobs.WithDSAllocationStore(allocDs))
	} else {
		blobOpts = append(blobOpts, blobs.WithAllocationStore(c.allocationStore))
	}

	claimStore := c.claimStore
	if claimStore == nil {
		claimDs := c.claimDatastore
		if claimDs == nil {
			claimDs = datastore.NewMapDatastore()
			log.Warn("Claim datastore not configured, using in-memory datastore")
		}
		closeFuncs = append(closeFuncs, func(context.Context) error { return claimDs.Close() })
		var err error
		claimStore, err = delegationstore.NewDsDelegationStore(claimDs)
		if err != nil {
			return nil, fmt.Errorf("creating claim store: %w", err)
		}
	}
	publisherStore := c.publisherStore
	if publisherStore == nil {
		publisherDs := c.publisherDatastore
		if publisherDs == nil {
			publisherDs = datastore.NewMapDatastore()
			log.Warn("Publisher datastore not configured, using in-memory datastore")
		}
		closeFuncs = append(closeFuncs, func(context.Context) error { return publisherDs.Close() })
		publisherStore = store.FromDatastore(publisherDs, store.WithMetadataContext(metadata.MetadataContext))
	}
	pubURL := c.publicURL
	if pubURL == (url.URL{}) {
		u, _ := url.Parse("http://localhost:3000")
		log.Warnf("Public URL not configured, using default: %s", u)
		pubURL = *u
	}

	receiptStore := c.receiptStore
	if receiptStore == nil {
		receiptDS := c.receiptDatastore
		if receiptDS == nil {
			receiptDS = datastore.NewMapDatastore()
			log.Warn("Receipt datastore not configured, using in-memory datastore")
		}
		closeFuncs = append(closeFuncs, func(context.Context) error { return receiptDS.Close() })
		var err error
		receiptStore, err = receiptstore.NewDsReceiptStore(receiptDS)
		if err != nil {
			return nil, fmt.Errorf("creating receipt store: %w", err)
		}
	}

	var pdpImpl pdp.PDP
	if c.pdpCfg == nil && c.pdpImpl == nil {
		blobStore := c.blobStore
		if blobStore == nil {
			blobStore = blobstore.NewMapBlobstore()
			log.Warn("Blob store not configured, using in-memory store")
		}

		blobOpts = append(blobOpts, blobs.WithBlobstore(blobStore))
		if c.blobsAccess != nil {
			blobOpts = append(blobOpts, blobs.WithAccess(c.blobsAccess))
		} else if c.blobsPublicURL != (url.URL{}) {
			blobOpts = append(blobOpts, blobs.WithPublicURLAccess(c.blobsPublicURL))
		} else {
			blobOpts = append(blobOpts, blobs.WithPublicURLAccess(pubURL))
		}

		if c.blobsPresigner != nil {
			blobOpts = append(blobOpts, blobs.WithPresigner(c.blobsPresigner))
		} else if c.blobsPublicURL != (url.URL{}) {
			blobOpts = append(blobOpts, blobs.WithPublicURLPresigner(id, c.blobsPublicURL))
		} else {
			blobOpts = append(blobOpts, blobs.WithPublicURLPresigner(id, pubURL))
		}
	} else {
		pdpImpl = c.pdpImpl
		if pdpImpl == nil {
			pdpService, err := pdp.NewRemote(c.pdpCfg, id, receiptStore)
			if err != nil {
				return nil, fmt.Errorf("creating pdp service: %w", err)
			}
			closeFuncs = append(closeFuncs, pdpService.Shutdown)
			startFuncs = append(startFuncs, pdpService.Startup)
			pdpImpl = pdpService
		}
	}

	var uploadServiceConnection client.Connection
	if c.uploadService == nil {
		channel := ucanhttp.NewHTTPChannel(presets.UploadServiceURL)
		conn, err := client.NewConnection(presets.UploadServiceDID, channel)
		if err != nil {
			return nil, fmt.Errorf("creating upload service connection: %w", err)
		}
		uploadServiceConnection = conn
	} else {
		uploadServiceConnection = c.uploadService
	}

	blobs, err := blobs.New(blobOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating blob service: %w", err)
	}

	peerAddr, err := maurl.FromURL(&pubURL)
	if err != nil {
		return nil, fmt.Errorf("parsing publisher url as multiaddr: %w", err)
	}

	claims, err := claims.New(
		id,
		claimStore,
		publisherStore,
		peerAddr,
		claims.WithPublisherDirectAnnounce(c.announceURLs...),
		claims.WithPublisherAnnounceAddress(c.publisherAnnouceAddr),
		claims.WithPublisherBlobAddress(c.publisherBlobAddress),
		claims.WithPublisherIndexingService(c.indexingService),
		claims.WithPublisherIndexingServiceProof(c.indexingServiceProofs...),
	)
	if err != nil {
		return nil, fmt.Errorf("creating claim service: %w", err)
	}

	if c.replicatorDB == nil {
		c.replicatorDB, err = sqlitedb.NewMemory()
		if err != nil {
			return nil, fmt.Errorf("creating in-memory replicator db: %w", err)
		}
	}

	// Create replication queue
	replicationQueue, err := jobqueue.New[*replicahandler.TransferRequest](
		"replication",
		c.replicatorDB,
		&serializer.JSON[*replicahandler.TransferRequest]{},
		jobqueue.WithLogger(log.With("queue", "replication")),
		jobqueue.WithMaxRetries(10),
		jobqueue.WithMaxWorkers(uint(runtime.NumCPU())),
	)
	if err != nil {
		return nil, fmt.Errorf("creating replication queue: %w", err)
	}

	repl, err := replicator.New(id, pdpImpl, blobs, claims, receiptStore, uploadServiceConnection, replicationQueue)
	if err != nil {
		return nil, fmt.Errorf("creating replicator service: %w", err)
	}

	// Register transfer task
	if err := repl.RegisterTransferTask(replicationQueue); err != nil {
		return nil, fmt.Errorf("registering replicator transfer task: %w", err)
	}

	// Queue lifecycle management
	var queueCtx context.Context
	var queueCancel context.CancelFunc
	startFuncs = append(startFuncs, func(ctx context.Context) error {
		queueCtx, queueCancel = context.WithCancel(context.Background())
		return replicationQueue.Start(queueCtx)
	})
	closeFuncs = append(closeFuncs, func(ctx context.Context) error {
		if queueCancel != nil {
			queueCancel()
		}
		return replicationQueue.Stop(ctx)
	})

	return &StorageService{
		id:            c.id,
		blobs:         blobs,
		claims:        claims,
		closeFuncs:    closeFuncs,
		startFuncs:    startFuncs,
		receiptStore:  receiptStore,
		pdp:           pdpImpl,
		replicator:    repl,
		uploadService: uploadServiceConnection,
	}, nil
}
