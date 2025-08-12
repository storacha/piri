package config

import (
	"fmt"
	"net/url"

	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/pkg/config/app"
)

type Services struct {
	ServicePrincipalMapping map[string]string `mapstructure:"principal_mapping" flag:"service-principal-mapping"`

	Indexer   IndexingService  `mapstructure:"indexer" validate:"required"`
	Upload    UploadService    `mapstructure:"upload" validate:"required"`
	Publisher PublisherService `mapstructure:"publisher" validate:"required"`
}

func (s Services) Validate() error {
	return validateConfig(s)
}

func (s Services) ToAppConfig(publicURL url.URL) (app.ExternalServicesConfig, error) {
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

type IndexingService struct {
	DID   string `mapstructure:"did" validate:"required" flag:"indexing-service-did"`
	URL   string `mapstructure:"url" validate:"required,url" flag:"indexing-service-url"`
	Proof string `mapstructure:"proof" flag:"indexing-service-proof"`
}

func (s *IndexingService) Validate() error {
	return validateConfig(s)
}

func (s *IndexingService) ToAppConfig() (app.IndexingServiceConfig, error) {
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
	}
	return out, nil
}

type UploadService struct {
	DID string `mapstructure:"did" validate:"required" flag:"upload-service-did"`
	URL string `mapstructure:"url" validate:"required,url" flag:"upload-service-url"`
}

func (s *UploadService) Validate() error {
	return validateConfig(s)
}

func (s *UploadService) ToAppConfig() (app.UploadServiceConfig, error) {
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

type PublisherService struct {
	AnnounceURLs []string `mapstructure:"ipni_announce_urls" validate:"required,min=1,dive,url" flag:"ipni-announce-urls"`
}

func (s *PublisherService) Validate() error {
	return validateConfig(s)
}

func (s *PublisherService) ToAppConfig(publicURL url.URL) (app.PublisherServiceConfig, error) {
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
	pieceAddr, err := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("piece/{blobCID}"))
	if err != nil {
		return app.PublisherServiceConfig{}, fmt.Errorf("creating piece multiaddr: %w", err)
	}
	return app.PublisherServiceConfig{
		PublicMaddr:   pubMaddr,
		AnnounceMaddr: pubMaddr,
		AnnounceURLs:  announceURLs,
		BlobMaddr:     multiaddr.Join(pdpEndpoint, pieceAddr),
	}, nil
}
