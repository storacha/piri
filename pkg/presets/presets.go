package presets

import (
	"fmt"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/did"
)

// Network represents the network the node will operate on
type Network string

const (
	ForgeProd   Network = "forge-prod"
	Prod        Network = "prod"
	Staging     Network = "staging"
	WarmStaging Network = "warm-staging"
)

var AvailableNetworks = []Network{ForgeProd, Prod, Staging, WarmStaging}

// String returns the string representation of the network
func (n Network) String() string {
	switch n {
	case ForgeProd, Prod, Staging, WarmStaging:
		return string(n)
	default:
		return "unknown"
	}
}

// ParseNetwork parses a string into a Network type
func ParseNetwork(s string) (Network, error) {
	switch s {
	case string(ForgeProd):
		return ForgeProd, nil
	case string(Prod):
		return Prod, nil
	case string(Staging):
		return Staging, nil
	case string(WarmStaging):
		return WarmStaging, nil
	default:
		return Network(""), fmt.Errorf("unknown network: %q (valid networks are: %q)", s, AvailableNetworks)
	}
}

// ServiceSettings holds the service configuration for a network
type ServiceSettings struct {
	IPNIAnnounceURLs        []url.URL
	IndexingServiceDID      did.DID
	IndexingServiceURL      *url.URL
	EgressTrackerServiceDID did.DID
	EgressTrackerServiceURL *url.URL
	UploadServiceURL        *url.URL
	UploadServiceDID        did.DID
	SigningServiceURL       *url.URL
	SigningServiceDID       did.DID
	RegistrarServiceURL     *url.URL
	PrincipalMapping        map[string]string
}

// SmartContractSettings holds the smart contract configuration for a network
type SmartContractSettings struct {
	Verifier         common.Address
	ProviderRegistry common.Address
	Service          common.Address
	ServiceView      common.Address
	ChainID          *big.Int
	PayerAddress     common.Address
}

// Preset holds all configuration presets for a network
type Preset struct {
	Services       ServiceSettings
	SmartContracts SmartContractSettings
}

// URL of the original and best IPNI node cid.contact.
var defaultIPNIAnnounceURL = lo.Must(url.Parse("https://cid.contact/announce"))

// Forge Production service preset values (default)
func forgeProdServiceSettings() ServiceSettings {
	forgeProdStorachaIPNIAnnounceURL := lo.Must(url.Parse("https://ipni.forge.storacha.network"))
	forgeProdIPNIAnnounceURLs := []url.URL{*defaultIPNIAnnounceURL, *forgeProdStorachaIPNIAnnounceURL}

	forgeProdIndexingServiceURL := lo.Must(url.Parse("https://indexer.forge.storacha.network/claims"))
	forgeProdIndexingServiceDID := lo.Must(did.Parse("did:web:indexer.forge.storacha.network"))

	forgeProdEgressTrackerServiceURL := lo.Must(url.Parse("https://etracker.forge.storacha.network"))
	forgeProdEgressTrackerServiceDID := lo.Must(did.Parse("did:web:etracker.forge.storacha.network"))

	forgeProdUploadServiceURL := lo.Must(url.Parse("https://up.forge.storacha.network"))
	forgeProdUploadServiceDID := lo.Must(did.Parse("did:web:up.forge.storacha.network"))

	forgeProdPrincipalMapping := map[string]string{
		forgeProdUploadServiceDID.String():        "did:key:z6MkgSttS3n3R56yGX2Eufvbwc58fphomhAsLoBCZpZJzQbr",
		forgeProdIndexingServiceDID.String():      "did:key:z6Mkj8WmJQRy5jEnFN97uuc2qsjFdsYCuD5wE384Z1AMCFN7",
		forgeProdEgressTrackerServiceDID.String(): "did:key:z6MkuGS213fJGP7qGRG8Pn9mffCDU2vXgnSRY4JL1sumgpFX",
	}

	forgeProdSigningServiceDID := lo.Must(did.Parse("did:web:signer.forge.storacha.network"))
	forgeProdSigningServiceURL := lo.Must(url.Parse("https://signer.forge.storacha.network"))

	forgeProdRegistrarServiceURL := lo.Must(url.Parse("https://registrar.forge.storacha.network"))

	return ServiceSettings{
		IPNIAnnounceURLs:        forgeProdIPNIAnnounceURLs,
		IndexingServiceURL:      forgeProdIndexingServiceURL,
		IndexingServiceDID:      forgeProdIndexingServiceDID,
		EgressTrackerServiceURL: forgeProdEgressTrackerServiceURL,
		EgressTrackerServiceDID: forgeProdEgressTrackerServiceDID,
		UploadServiceURL:        forgeProdUploadServiceURL,
		UploadServiceDID:        forgeProdUploadServiceDID,
		SigningServiceURL:       forgeProdSigningServiceURL,
		SigningServiceDID:       forgeProdSigningServiceDID,
		RegistrarServiceURL:     forgeProdRegistrarServiceURL,
		PrincipalMapping:        forgeProdPrincipalMapping,
	}
}

// Warm Staging service preset values
func warmStagingServiceSettings() ServiceSettings {
	warmStagingStorachaIPNIAnnounceURL := lo.Must(url.Parse("https://staging.ipni.warm.storacha.network"))
	warmStagingIPNIAnnounceURLs := []url.URL{*defaultIPNIAnnounceURL, *warmStagingStorachaIPNIAnnounceURL}

	warmStagingIndexingServiceURL := lo.Must(url.Parse("https://staging.indexer.warm.storacha.network/claims"))
	warmStagingIndexingServiceDID := lo.Must(did.Parse("did:web:staging.indexer.warm.storacha.network"))

	warmStagingEgressTrackerServiceURL := lo.Must(url.Parse("https://staging.etracker.warm.storacha.network"))
	warmStagingEgressTrackerServiceDID := lo.Must(did.Parse("did:web:staging.etracker.warm.storacha.network"))

	warmStagingUploadServiceURL := lo.Must(url.Parse("https://staging.up.warm.storacha.network"))
	warmStagingUploadServiceDID := lo.Must(did.Parse("did:web:staging.up.warm.storacha.network"))

	warmStagingPrincipalMapping := map[string]string{
		warmStagingUploadServiceDID.String():        "did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v",
		warmStagingIndexingServiceDID.String():      "did:key:z6Mkr4QkdinnXQmJ9JdnzwhcEjR8nMnuVPEwREyh9jp2Pb7k",
		warmStagingEgressTrackerServiceDID.String(): "did:key:z6Mkqv9fjGQpNKQdgUxkq2VYH2nKiKZiGPxbtYjhJBz8wfAn",
	}

	warmStagingSigningServiceDID := lo.Must(did.Parse("did:web:staging.signer.warm.storacha.network"))
	warmStagingSigningServiceURL := lo.Must(url.Parse("https://staging.signer.warm.storacha.network"))

	warmStagingRegistrarServiceURL := lo.Must(url.Parse("https://staging.registrar.warm.storacha.network"))

	return ServiceSettings{
		IPNIAnnounceURLs:        warmStagingIPNIAnnounceURLs,
		IndexingServiceURL:      warmStagingIndexingServiceURL,
		IndexingServiceDID:      warmStagingIndexingServiceDID,
		EgressTrackerServiceURL: warmStagingEgressTrackerServiceURL,
		EgressTrackerServiceDID: warmStagingEgressTrackerServiceDID,
		UploadServiceURL:        warmStagingUploadServiceURL,
		UploadServiceDID:        warmStagingUploadServiceDID,
		SigningServiceURL:       warmStagingSigningServiceURL,
		SigningServiceDID:       warmStagingSigningServiceDID,
		RegistrarServiceURL:     warmStagingRegistrarServiceURL,
		PrincipalMapping:        warmStagingPrincipalMapping,
	}
}

// Staging service preset values
func stagingServiceSettings() ServiceSettings {
	stagingStorachaIPNIAnnounceURL := lo.Must(url.Parse("https://staging.ipni.storacha.network"))
	stagingIPNIAnnounceURLs := []url.URL{*defaultIPNIAnnounceURL, *stagingStorachaIPNIAnnounceURL}

	stagingIndexingServiceURL := lo.Must(url.Parse("https://staging.indexer.storacha.network/claims"))
	stagingIndexingServiceDID := lo.Must(did.Parse("did:web:staging.indexer.storacha.network"))

	stagingEgressTrackerServiceURL := lo.Must(url.Parse("https://staging.etracker.storacha.network"))
	stagingEgressTrackerServiceDID := lo.Must(did.Parse("did:web:staging.etracker.storacha.network"))

	stagingUploadServiceURL := lo.Must(url.Parse("https://staging.up.storacha.network"))
	stagingUploadServiceDID := lo.Must(did.Parse("did:web:staging.up.storacha.network"))

	stagingPrincipalMapping := map[string]string{
		stagingUploadServiceDID.String():        "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		stagingIndexingServiceDID.String():      "did:key:z6MkszJLAZ1tCHUTfDMKj9srMYA9zzLiPeMXijvmeECUScTZ",
		stagingEgressTrackerServiceDID.String(): "did:key:z6MkmSQ8ZZQffBaQo5mr3fArRsDxKPXyPQFroiB1H9EbAHod",
	}

	return ServiceSettings{
		IPNIAnnounceURLs:        stagingIPNIAnnounceURLs,
		IndexingServiceURL:      stagingIndexingServiceURL,
		IndexingServiceDID:      stagingIndexingServiceDID,
		EgressTrackerServiceURL: stagingEgressTrackerServiceURL,
		EgressTrackerServiceDID: stagingEgressTrackerServiceDID,
		UploadServiceURL:        stagingUploadServiceURL,
		UploadServiceDID:        stagingUploadServiceDID,
		SigningServiceURL:       nil,
		SigningServiceDID:       did.Undef,
		RegistrarServiceURL:     nil,
		PrincipalMapping:        stagingPrincipalMapping,
	}
}

// Production service preset values
func prodServiceSettings() ServiceSettings {
	prodStorachaIPNIAnnounceURL := lo.Must(url.Parse("https://ipni.storacha.network"))
	prodIPNIAnnounceURLs := []url.URL{*defaultIPNIAnnounceURL, *prodStorachaIPNIAnnounceURL}

	prodIndexingServiceURL := lo.Must(url.Parse("https://indexer.storacha.network/claims"))
	prodIndexingServiceDID := lo.Must(did.Parse("did:web:indexer.storacha.network"))

	prodEgressTrackerServiceURL := lo.Must(url.Parse("https://etracker.storacha.network"))
	prodEgressTrackerServiceDID := lo.Must(did.Parse("did:web:etracker.storacha.network"))

	prodUploadServiceURL := lo.Must(url.Parse("https://up.storacha.network"))
	prodUploadServiceDID := lo.Must(did.Parse("did:web:up.storacha.network"))

	prodPrincipalMapping := map[string]string{
		prodUploadServiceDID.String():        "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
		prodIndexingServiceDID.String():      "did:key:z6MkqMSJxrjzvpqmP3kZhk7eCasBK6DX1jaVaG7wD72LYRm7",
		prodEgressTrackerServiceDID.String(): "did:key:z6MkiVMkL8MSzqi3iqFj2AjofQfL8wH6p7AcB2w34mKbyWfF",
	}

	return ServiceSettings{
		IPNIAnnounceURLs:        prodIPNIAnnounceURLs,
		IndexingServiceURL:      prodIndexingServiceURL,
		IndexingServiceDID:      prodIndexingServiceDID,
		EgressTrackerServiceURL: prodEgressTrackerServiceURL,
		EgressTrackerServiceDID: prodEgressTrackerServiceDID,
		UploadServiceURL:        prodUploadServiceURL,
		UploadServiceDID:        prodUploadServiceDID,
		SigningServiceURL:       nil,
		SigningServiceDID:       did.Undef,
		RegistrarServiceURL:     nil,
		PrincipalMapping:        prodPrincipalMapping,
	}
}

// Contract settings for calibration network
var calibnetSettings = SmartContractSettings{
	// PDPVerifier contract address (see https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts)
	Verifier: common.HexToAddress("0x85e366Cf9DD2c0aE37E963d9556F5f4718d6417C"),
	// This contract and its address are owned by storacha
	ProviderRegistry: common.HexToAddress("0x6A96aaB210B75ee733f0A291B5D8d4A053643979"),
	// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
	Service:     common.HexToAddress("0x0c6875983B20901a7C3c86871f43FdEE77946424"),
	ServiceView: common.HexToAddress("0xEAD67d775f36D1d2894854D20e042C77A3CC20a5"),
	// Filecoin calibration chain ID
	ChainID: big.NewInt(314159),
	// PayerAddress is the Storacha Owned address that pays SPs
	PayerAddress: common.HexToAddress("0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD"),
}

// Contract settings for mainnet
var mainnetSettings = SmartContractSettings{
	// PDPVerifier contract address (see https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts)
	Verifier: common.HexToAddress("0xBADd0B92C1c71d02E7d520f64c0876538fa2557F"),
	// This contract and its address are owned by storacha
	ProviderRegistry: common.HexToAddress("0xf55dDbf63F1b55c3F1D4FA7e339a68AB7b64A5eB"),
	// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
	Service:     common.HexToAddress("0x56e53c5e7F27504b810494cc3b88b2aa0645a839"),
	ServiceView: common.HexToAddress("0x778Bbb8F50d759e2AA03ab6FAEF830Ca329AFF9D"),
	// Filecoin mainnet chain ID
	ChainID: big.NewInt(314),
	// PayerAddress is the Storacha Owned address that pays SPs
	PayerAddress: common.HexToAddress("0x3c1ae7a70a2b51458fcb7927fd77aae408a1b857"),
}

// GetPreset returns the complete preset configuration for a given network
func GetPreset(network Network) (Preset, error) {
	switch network {
	case ForgeProd:
		return Preset{
			Services:       forgeProdServiceSettings(),
			SmartContracts: mainnetSettings,
		}, nil
	case Prod:
		return Preset{
			Services:       prodServiceSettings(),
			SmartContracts: mainnetSettings,
		}, nil
	case Staging:
		return Preset{
			Services:       stagingServiceSettings(),
			SmartContracts: calibnetSettings,
		}, nil
	case WarmStaging:
		return Preset{
			Services:       warmStagingServiceSettings(),
			SmartContracts: calibnetSettings,
		}, nil
	default:
		return Preset{}, fmt.Errorf("unknown network: %s", network)
	}
}
