package config

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/curio"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("services-config")

func ProvidePrincipal(cfg app.Config) principal.Signer {
	return cfg.ID
}

// Service URL and DID providers
func ProvideUploadServiceDID(cfg app.Config) did.DID {
	return cfg.UploadServiceDID
}

func ProvideUploadServiceURL(cfg app.Config) *url.URL {
	return cfg.UploadServiceURL
}

func ProvideIndexingServiceDID(cfg app.Config) did.DID {
	return cfg.IndexingServiceDID
}

func ProvideIndexingServiceURL(cfg app.Config) *url.URL {
	return cfg.IndexingServiceURL
}

func ProvideServicePrincipalMapping(cfg app.Config) map[string]string {
	return cfg.ServicePrincipalMapping
}

// ProvidePublicURLPreSigner creates a presigner for the public URL
func ProvidePublicURLPreSigner(cfg app.Config) (presigner.RequestPresigner, error) {
	id := cfg.ID
	accessKeyID := id.DID().String()
	idDigest, err := multihash.Sum(id.Encode(), multihash.SHA2_256, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate access key ID with multihash: %w", err)
	}
	secretAccessKey := digestutil.Format(idDigest)
	ps, err := presigner.NewS3RequestPresigner(accessKeyID, secretAccessKey, *cfg.PublicURL, "blob")
	if err != nil {
		return nil, fmt.Errorf("could not create presigner: %w", err)
	}
	return ps, nil
}

func ProvidePublicURLAccess(cfg app.Config) (access.Access, error) {
	accessURL := cfg.PublicURL
	accessURL.Path = "/blob"
	return access.NewPatternAccess(fmt.Sprintf("%s/{blob}", accessURL.String()))
}

func ProvidePeerMultiAddress(cfg app.Config) (multiaddr.Multiaddr, error) {
	return maurl.FromURL(cfg.PublicURL)
}

// ProvideUploadServiceConnection provides the upload service connection
// This is provided with name:"upload" to distinguish from indexing service connection
func ProvideUploadServiceConnection(cfg app.Config) (client.Connection, error) {
	channel := ucanhttp.NewHTTPChannel(cfg.UploadServiceURL)
	conn, err := client.NewConnection(cfg.UploadServiceDID, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create upload connection: %w", err)
	}
	return conn, nil
}

func ProvideIndexingServiceProofs(cfg app.Config) (delegation.Proofs, error) {
	return delegation.Proofs{cfg.IndexingServiceProofs}, nil
}

func ProvideIPNIAnnounceURLs(cfg app.Config) ([]url.URL, error) {
	return cfg.AnnounceURLs, nil
}

// ProvideIndexingServiceConnection provides the indexing service connection
// This is provided with name:"indexing" to distinguish from upload service connection
func ProvideIndexingServiceConnection(cfg app.Config) (client.Connection, error) {
	channel := ucanhttp.NewHTTPChannel(cfg.IndexingServiceURL)
	conn, err := client.NewConnection(cfg.IndexingServiceDID, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create indexing connection: %w", err)
	}
	return conn, nil
}

func ProvidePDPService(cfg app.Config, receiptStore receiptstore.ReceiptStore, lc fx.Lifecycle) (pdp.PDP, error) {
	if cfg.PDPConfig == nil {
		log.Info("PDP Service is disabled. Operating w/o PDP")
		return nil, nil
	}

	var ds datastore.Datastore
	if cfg.DataDir == "" {
		log.Warn("no data directory provided, using memory aggregator pdp store")
		ds = datastore.NewMapDatastore()
	} else {
		aggregatorDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "aggregator"))
		if err != nil {
			return nil, fmt.Errorf("could not create aggregator directory: %w", err)
		}
		ds, err = leveldb.NewDatastore(aggregatorDir, nil)
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Info("closing aggregator pdp datastore")
				return ds.Close()
			},
		})
	}

	aggJobQueueDir, err := app.Mkdirp(filepath.Join(cfg.DataDir, "aggregator"), "jobqueue")
	if err != nil {
		return nil, err
	}
	curioAuth, err := curio.CreateCurioJWTAuthHeader("storacha", cfg.ID)
	if err != nil {
		return nil, fmt.Errorf("generating curio JWT: %w", err)
	}
	curioClient := curio.New(http.DefaultClient, cfg.PDPConfig.Endpoint, curioAuth)

	pdpService, err := pdp.NewRemotePDPService(
		ds,
		filepath.Join(aggJobQueueDir, "queue.db"),
		curioClient,
		cfg.PDPConfig.ProofSet,
		cfg.ID,
		receiptStore,
	)
	if err != nil {
		return nil, fmt.Errorf("creating pdp service: %w", err)
	}
	return pdpService, nil
}
