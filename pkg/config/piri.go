package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/config/app"
)

var log = logging.Logger("config")

type Identity struct {
	KeyFile string `mapstructure:"key_file" validate:"required" flag:"key-file"`
}

func (i Identity) Validate() error {
	return validateConfig(i)
}

func (i Identity) ToAppConfig() (app.IdentityConfig, error) {
	id, err := lib.SignerFromEd25519PEMFile(i.KeyFile)
	if err != nil {
		return app.IdentityConfig{}, err
	}
	return app.IdentityConfig{
		Signer: id,
	}, nil
}

type Repo struct {
	DataDir string `mapstructure:"data_dir" validate:"required" flag:"data-dir"`
	TempDir string `mapstructure:"temp_dir" validate:"required" flag:"temp-dir"`
}

func (r Repo) Validate() error {
	return validateConfig(r)
}

func (r Repo) ToAppConfig() (app.StorageConfig, error) {
	if r.DataDir == "" {
		// Return empty config for memory stores
		return app.StorageConfig{}, nil
	}

	// Ensure directories exist
	if err := os.MkdirAll(r.DataDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}
	if err := os.MkdirAll(r.TempDir, 0755); err != nil {
		return app.StorageConfig{}, err
	}

	out := app.StorageConfig{
		DataDir: r.DataDir,
		TempDir: r.TempDir,
		Aggregator: app.AggregatorStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "aggregator", "datastore"),
			DBPath:   filepath.Join(r.DataDir, "aggregator", "jobqueue", "jobqueue.db"),
		},
		Blobs: app.BlobStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "blobs"),
			TempDir:  filepath.Join(r.TempDir, "storage"),
		},
		Claims: app.ClaimStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "claim"),
		},
		Publisher: app.PublisherStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "publisher"),
		},
		Receipts: app.ReceiptStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "receipt"),
		},
		Allocations: app.AllocationStorageConfig{
			StoreDir: filepath.Join(r.DataDir, "allocation"),
		},
		Replicator: app.ReplicatorStorageConfig{
			DBPath: filepath.Join(r.DataDir, "replicator", "replicator.db"),
		},
		KeyStore: app.KeyStoreConfig{
			StoreDir: filepath.Join(r.DataDir, "wallet"),
		},
		StashStore: app.StashStoreConfig{
			StoreDir: filepath.Join(r.DataDir, "pdp"),
		},
		SchedulerStorage: app.SchedulerConfig{
			DBPath: filepath.Join(r.DataDir, "pdp", "state", "scheduler.db"),
		},
		PDPStore: app.PDPStoreConfig{
			StoreDir: filepath.Join(r.DataDir, "pdp", "datastore"),
		},
	}

	if err := os.MkdirAll(filepath.Dir(out.Aggregator.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating aggregator db: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out.Replicator.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating replicator db: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out.SchedulerStorage.DBPath), 0755); err != nil {
		return app.StorageConfig{}, fmt.Errorf("creating scheduler db: %w", err)
	}

	return out, nil

}

type Server struct {
	Port      uint   `mapstructure:"port" validate:"required,min=1,max=65535" flag:"port"`
	Host      string `mapstructure:"host" validate:"required" flag:"host"`
	PublicURL string `mapstructure:"public_url" validate:"omitempty,url" flag:"public-url"`
}

func (s Server) Validate() error {
	return validateConfig(s)
}

func (s Server) ToAppConfig() (app.ServerConfig, error) {
	var err error
	var publicURL *url.URL
	if s.PublicURL != "" {
		publicURL, err = url.Parse(s.PublicURL)
		if err != nil {
			return app.ServerConfig{}, fmt.Errorf("parsing public URL: %w", err)
		}
	} else {
		log.Warn("public URL not set, using http://host+port")
		publicURL, err = url.Parse(fmt.Sprintf("http://%s:%d", s.Host, s.Port))
		if err != nil {
			return app.ServerConfig{}, fmt.Errorf("creating default public URL: %w", err)
		}
	}

	return app.ServerConfig{
		Host:      s.Host,
		Port:      s.Port,
		PublicURL: publicURL,
	}, nil
}

type Services struct {
	ServicePrincipalMapping map[string]string `mapstructure:"service_principal_mapping" flag:"service-principal-mapping"`

	Indexer   IndexingService  `mapstructure:"indexer,squash" validate:"required"`
	Publisher PublisherService `mapstructure:"publisher,squash" validate:"required"`
	Upload    UploadService    `mapstructure:"upload,squash" validate:"required"`
}

func (s Services) Validate() error {
	return validateConfig(s)
}

func (s Services) ToAppConfig(publicURL *url.URL) (app.ServicesConfig, error) {
	var out app.ServicesConfig

	//
	// Upload service
	//
	uploadDID, err := did.Parse(s.Upload.DID)
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("parsing upload service DID: %w", err)
	}
	uploadURL, err := url.Parse(s.Upload.URL)
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("parsing upload service URL: %w", err)
	}
	uploadChannel := ucanhttp.NewHTTPChannel(uploadURL)
	uploadConn, err := client.NewConnection(uploadDID, uploadChannel)
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("creating upload service connection: %w", err)
	}
	out.UploadService = app.ServiceConnectionConfig{
		Connection: uploadConn,
	}

	//
	// Indexing service
	//
	indexingDID, err := did.Parse(s.Indexer.DID)
	if err != nil {
		return out, fmt.Errorf("parsing indexing service DID: %w", err)
	}
	indexingURL, err := url.Parse(s.Indexer.URL)
	if err != nil {
		return out, fmt.Errorf("parsing indexing service URL: %w", err)
	}
	indexingChannel := ucanhttp.NewHTTPChannel(indexingURL)
	indexingConn, err := client.NewConnection(indexingDID, indexingChannel)
	if err != nil {
		return out, fmt.Errorf("creating indexing service connection: %w", err)
	}
	out.IndexingService = app.IndexingServiceConfig{
		Connection: indexingConn,
	}
	// Parse indexing service proofs if provided
	if s.Indexer.Proof != "" {
		dlg, err := delegation.Parse(s.Indexer.Proof)
		if err != nil {
			return out, fmt.Errorf("parsing indexing service proof: %w", err)
		}
		out.IndexingService.Proofs = delegation.Proofs{delegation.FromDelegation(dlg)}
	}

	//
	// Publisher service
	//
	pubMaddr, err := maurl.FromURL(publicURL)
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("converting public URL to multiaddr: %w", err)
	}

	// Parse IPNI announce URLs
	var announceURLs []url.URL
	for _, s := range s.Publisher.AnnounceURLs {
		u, err := url.Parse(s)
		if err != nil {
			return app.ServicesConfig{}, fmt.Errorf("parsing IPNI announce URL %s: %w", s, err)
		}
		announceURLs = append(announceURLs, *u)
	}

	pdpEndpoint, err := maurl.FromURL(publicURL)
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("converting PDP URL to multiaddr: %w", err)
	}
	pieceAddr, err := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("piece/{blobCID}"))
	if err != nil {
		return app.ServicesConfig{}, fmt.Errorf("creating piece multiaddr: %w", err)
	}
	out.Publisher = app.PublisherConfig{
		PublicMaddr:   pubMaddr,
		AnnounceMaddr: pubMaddr,
		AnnounceURLs:  announceURLs,
		BlobMaddr:     multiaddr.Join(pdpEndpoint, pieceAddr),
	}

	//
	// service mapping
	//
	out.ServicePrincipalMapping = s.ServicePrincipalMapping

	// nil because this config is for single process, and there won't be a different endpoint for the PDP server, its the same
	// endpoint as the public URL
	out.PDPServer = nil

	return out, nil
}

type Piri struct {
	Identity `mapstructure:"identity,squash" validate:"required"`
	Repo     `mapstructure:"repo,squash" validate:"required"`
	Server   `mapstructure:"server,squash" validate:"required"`
	Services `mapstructure:"services,squash" validate:"required"`

	LotusEndpoint       string `mapstructure:"lotus_endpoint" validate:"required,url" flag:"lotus-endpoint"`
	OwnerAddress        string `mapstructure:"owner_address" validate:"required" flag:"owner-address"`
	RecordKeeperAddress string `mapstructure:"contract_address" validate:"required" flag:"contract-address"`

	ProofSet uint64 `mapstructure:"proof_set" flag:"proof-set"`
}

func (p Piri) Validate() error {
	return validateConfig(p)
}

func (p Piri) ToAppConfig() (app.AppConfig, error) {
	var (
		err error
		out app.AppConfig
	)

	out.Identity, err = p.Identity.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting identity to app config: %s", err)
	}

	out.Server, err = p.Server.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting server config to app config: %s", err)
	}

	out.Storage, err = p.Repo.ToAppConfig()
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting repo to app config: %s", err)
	}

	out.Services, err = p.Services.ToAppConfig(out.Server.PublicURL)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting services to app config: %s", err)
	}

	lotusEndpoint, err := url.Parse(p.LotusEndpoint)
	if err != nil {
		return app.AppConfig{}, fmt.Errorf("converting Lotus URL to app config: %s", err)
	}

	if !common.IsHexAddress(p.RecordKeeperAddress) {
		return app.AppConfig{}, fmt.Errorf("invalid record keeper address: %s", p.RecordKeeperAddress)
	}

	if !common.IsHexAddress(p.OwnerAddress) {
		return app.AppConfig{}, fmt.Errorf("invalid owner address: %s", p.OwnerAddress)
	}

	out.Blockchain = app.BlockchainConfig{
		OwnerAddr:        common.HexToAddress(p.OwnerAddress),
		RecordKeeperAddr: common.HexToAddress(p.RecordKeeperAddress),
		LotusEndpoint:    lotusEndpoint,
	}
	return out, nil
}
