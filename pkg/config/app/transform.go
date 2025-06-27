package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/presets"
)

var log = logging.Logger("app-config")

// TransformUCANConfig transforms user configuration into application-ready configuration
func TransformUCANConfig(cfg config.UCANServer) (Config, error) {
	storageCfg := Config{
		ServicePrincipalMapping: cfg.ServicePrincipalMapping,
		DataDir:                 cfg.DataDir,
		TempDir:                 cfg.TempDir,
	}

	// Create directories if specified
	if storageCfg.DataDir != "" {
		dir, err := Mkdirp(storageCfg.DataDir)
		if err != nil {
			return storageCfg, fmt.Errorf("could not create data directory: %s", err)
		}
		storageCfg.DataDir = dir
	}

	if storageCfg.TempDir != "" {
		dir, err := Mkdirp(storageCfg.TempDir)
		if err != nil {
			return storageCfg, fmt.Errorf("could not create temp directory: %s", err)
		}
		storageCfg.TempDir = dir
	}

	// Parse identity
	id, err := cliutil.ReadPrivateKeyFromPEM(cfg.KeyFile)
	if err != nil {
		return storageCfg, fmt.Errorf("loading identity: %w", err)
	}
	storageCfg.ID = id

	// Parse URLs
	if cfg.PublicURL != "" {
		u, err := url.Parse(cfg.PublicURL)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing public URL: %w", err)
		}
		storageCfg.PublicURL = u
	} else {
		storageCfg.PublicURL, err = url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port))
		if err != nil {
			return storageCfg, fmt.Errorf("parsing public URL: %w", err)
		}
		log.Warnf("public URL not configured, using host:port: %s", storageCfg.PublicURL)
	}

	// Parse service URLs and DIDs
	if cfg.UploadServiceURL != "" {
		u, err := url.Parse(cfg.UploadServiceURL)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing upload service URL: %w", err)
		}
		storageCfg.UploadServiceURL = u
	}

	if cfg.UploadServiceDID != "" {
		d, err := did.Parse(cfg.UploadServiceDID)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing upload service DID: %w", err)
		}
		storageCfg.UploadServiceDID = d
	}

	if cfg.IndexingServiceURL != "" {
		u, err := url.Parse(cfg.IndexingServiceURL)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing indexing service URL: %w", err)
		}
		storageCfg.IndexingServiceURL = u
	}

	if cfg.IndexingServiceDID != "" {
		d, err := did.Parse(cfg.IndexingServiceDID)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing indexing service DID: %w", err)
		}
		storageCfg.IndexingServiceDID = d
	}

	// Parse announce URLs
	for _, urlStr := range cfg.IPNIAnnounceURLs {
		u, err := url.Parse(urlStr)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing announce URL %s: %w", urlStr, err)
		}
		storageCfg.AnnounceURLs = append(storageCfg.AnnounceURLs, *u)
	}

	// Parse indexing service proofs if provided
	if cfg.IndexingServiceProof != "" {
		dlg, err := delegation.Parse(cfg.IndexingServiceProof)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing indexing service proof: %w", err)
		}
		storageCfg.IndexingServiceProofs = delegation.FromDelegation(dlg)
	}

	// Parse PDP configuration
	if cfg.PDPServerURL != "" {
		u, err := url.Parse(cfg.PDPServerURL)
		if err != nil {
			return storageCfg, fmt.Errorf("parsing PDP server URL: %w", err)
		}
		storageCfg.PDPConfig = &PDPConfig{
			Endpoint: u,
			ProofSet: cfg.ProofSet,
		}
	}

	// Apply defaults to fill in any missing values
	storageCfg = ApplyDefaults(storageCfg)

	return storageCfg, nil
}

// ApplyDefaults applies default values to a Config struct where values are not set.
// This ensures all required fields have sensible defaults.
func ApplyDefaults(cfg Config) Config {
	// Apply service URL and DID defaults
	if cfg.UploadServiceURL == nil {
		cfg.UploadServiceURL = presets.UploadServiceURL
	}
	if !cfg.UploadServiceDID.Defined() {
		cfg.UploadServiceDID = presets.UploadServiceDID
	}
	if cfg.IndexingServiceURL == nil {
		cfg.IndexingServiceURL = presets.IndexingServiceURL
	}
	if !cfg.IndexingServiceDID.Defined() {
		cfg.IndexingServiceDID = presets.IndexingServiceDID
	}

	// Apply IPNI announce URLs default
	if len(cfg.AnnounceURLs) == 0 {
		cfg.AnnounceURLs = presets.IPNIAnnounceURLs
	}

	// Apply service principal mapping defaults
	if cfg.ServicePrincipalMapping == nil {
		cfg.ServicePrincipalMapping = presets.PrincipalMapping
	}

	if cfg.PublicURL == nil {
		cfg.PublicURL = presets.UCANServerPublicURL
	}

	return cfg
}

// Mkdirp creates a directory and all parent directories if they don't exist
func Mkdirp(dirpath ...string) (string, error) {
	dir := filepath.Join(dirpath...)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("creating directory: %s: %w", dir, err)
	}
	return dir, nil
}
