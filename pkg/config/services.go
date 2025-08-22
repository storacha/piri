package config

import (
	"fmt"
	"net/url"

	"github.com/ipni/go-libipni/maurl"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/lib"
	"github.com/storacha/piri/pkg/config/app"
)

type ServicesConfig struct {
	ServicePrincipalMapping map[string]string `mapstructure:"principal_mapping" flag:"service-principal-mapping" toml:"principal_mapping,omitempty"`

	Indexer   IndexingServiceConfig  `mapstructure:"indexer" validate:"required" toml:"indexer,omitempty"`
	Upload    UploadServiceConfig    `mapstructure:"upload" validate:"required" toml:"upload,omitempty"`
	Publisher PublisherServiceConfig `mapstructure:"publisher" validate:"required" toml:"publisher,omitempty"`
}

func (s ServicesConfig) Validate() error {
	return validateConfig(s)
}

func (s ServicesConfig) ToAppConfig(publicURL url.URL) (app.ExternalServicesConfig, error) {
	var (
		out app.ExternalServicesConfig
		err error
	)

	out.Upload, err = s.Upload.ToAppConfig()
	if err != nil {
		return app.ExternalServicesConfig{}, fmt.Errorf("creating upload service app config: %w", err)
	}
	out.Indexer, err = s.Indexer.ToAppConfig()
	if err != nil {
		return app.ExternalServicesConfig{}, fmt.Errorf("creating indexing service app config: %w", err)
	}

	out.Publisher, err = s.Publisher.ToAppConfig(publicURL)
	if err != nil {
		return app.ExternalServicesConfig{}, fmt.Errorf("creating publisher service app config: %w", err)
	}

	if s.ServicePrincipalMapping != nil {
		out.PrincipalMapping = s.ServicePrincipalMapping
	} else {
		out.PrincipalMapping = make(map[string]string)
	}

	return out, nil
}

type IndexingServiceConfig struct {
	DID   string `mapstructure:"did" validate:"required" flag:"indexing-service-did" toml:"did,omitempty"`
	URL   string `mapstructure:"url" validate:"required,url" flag:"indexing-service-url" toml:"url,omitempty"`
	Proof string `mapstructure:"proof" flag:"indexing-service-proof" toml:"proof,omitempty"`
}

func (s *IndexingServiceConfig) Validate() error {
	return validateConfig(s)
}

func (s *IndexingServiceConfig) ToAppConfig() (app.IndexingServiceConfig, error) {
	sdid, err := did.Parse(s.DID)
	if err != nil {
		return app.IndexingServiceConfig{}, fmt.Errorf("parsing indexing service DID: %w", err)
	}
	surl, err := url.Parse(s.URL)
	if err != nil {
		return app.IndexingServiceConfig{}, fmt.Errorf("parsing indexing service URL: %w", err)
	}
	schannel := ucanhttp.NewHTTPChannel(surl)
	sconn, err := client.NewConnection(sdid, schannel)
	if err != nil {
		return app.IndexingServiceConfig{}, fmt.Errorf("creating indexing service connection: %w", err)
	}
	out := app.IndexingServiceConfig{
		Connection: sconn,
	}
	// Parse indexing service proofs if provided
	if s.Proof != "" {
		dlg, err := delegation.Parse(s.Proof)
		if err != nil {
			return app.IndexingServiceConfig{}, fmt.Errorf("parsing indexing service proof: %w", err)
		}
		out.Proofs = delegation.Proofs{delegation.FromDelegation(dlg)}
	} else {
		// TODO(forrest): in the event a node is run without an indexing service proof, it will
		// almost always fail to index...obviously.
		// The TODO here is one of:
		//   1. Fail to start the node (will be annoying for testing
		//   2. Return an app config with a nil indexing service connection
		//      dependencies of this config are usually fine with a nil connection, as they check it before use.
		log.Warn("no indexing service proof provided, indexing will likely fail, please provide indexing proof")
	}
	return out, nil
}

type UploadServiceConfig struct {
	DID string `mapstructure:"did" validate:"required" flag:"upload-service-did" toml:"did,omitempty"`
	URL string `mapstructure:"url" validate:"required,url" flag:"upload-service-url" toml:"url,omitempty"`
}

func (s *UploadServiceConfig) Validate() error {
	return validateConfig(s)
}

func (s *UploadServiceConfig) ToAppConfig() (app.UploadServiceConfig, error) {
	sdid, err := did.Parse(s.DID)
	if err != nil {
		return app.UploadServiceConfig{}, fmt.Errorf("parsing indexing service DID: %w", err)
	}
	surl, err := url.Parse(s.URL)
	if err != nil {
		return app.UploadServiceConfig{}, fmt.Errorf("parsing indexing service URL: %w", err)
	}
	schannel := ucanhttp.NewHTTPChannel(surl)
	sconn, err := client.NewConnection(sdid, schannel)
	if err != nil {
		return app.UploadServiceConfig{}, fmt.Errorf("creating indexing service connection: %w", err)
	}
	return app.UploadServiceConfig{
		Connection: sconn,
	}, nil
}

type PublisherServiceConfig struct {
	AnnounceURLs []string `mapstructure:"ipni_announce_urls" validate:"required,min=1,dive,url" flag:"ipni-announce-urls" toml:"ipni_announce_urls,omitempty"`
}

func (s *PublisherServiceConfig) Validate() error {
	return validateConfig(s)
}

func (s *PublisherServiceConfig) ToAppConfig(publicURL url.URL) (app.PublisherServiceConfig, error) {
	pubMaddr, err := maurl.FromURL(&publicURL)
	if err != nil {
		return app.PublisherServiceConfig{}, fmt.Errorf("converting public URL to multiaddr: %w", err)
	}

	// Parse IPNI announce URLs
	var announceURLs []url.URL
	for _, s := range s.AnnounceURLs {
		u, err := url.Parse(s)
		if err != nil {
			return app.PublisherServiceConfig{}, fmt.Errorf("parsing IPNI announce URL %s: %w", s, err)
		}
		announceURLs = append(announceURLs, *u)
	}

	pdpEndpoint, err := maurl.FromURL(&publicURL)
	if err != nil {
		return app.PublisherServiceConfig{}, fmt.Errorf("converting PDP URL to multiaddr: %w", err)
	}
	blobMaddr, err := lib.JoinHTTPPath(pdpEndpoint, "piece/{blobCID}")
	if err != nil {
		return app.PublisherServiceConfig{}, fmt.Errorf("creating blob multiaddr: %w", err)
	}
	return app.PublisherServiceConfig{
		PublicMaddr:   pubMaddr,
		AnnounceMaddr: pubMaddr,
		AnnounceURLs:  announceURLs,
		BlobMaddr:     blobMaddr,
	}, nil
}
