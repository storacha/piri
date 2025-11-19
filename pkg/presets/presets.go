package presets

import (
	"fmt"
	"math/big"
	"net/url"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/did"
)

// Network represents the network the node will operate on
type Network int

const (
	ForgeProd Network = iota
	Prod
	Staging
	WarmStaging
)

// String returns the string representation of the network
func (n Network) String() string {
	switch n {
	case ForgeProd:
		return "forge-prod"
	case Prod:
		return "prod"
	case Staging:
		return "staging"
	case WarmStaging:
		return "warm-staging"
	default:
		return "unknown"
	}
}

// ParseNetwork parses a string into a Network type
func ParseNetwork(s string) (Network, error) {
	switch s {
	case "forge-prod":
		return ForgeProd, nil
	case "prod":
		return Prod, nil
	case "staging":
		return Staging, nil
	case "warm-staging":
		return WarmStaging, nil
	default:
		return ForgeProd, fmt.Errorf("unknown network: %s", s)
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
	PrincipalMapping        map[string]string
	SigningServiceEndpoint  *url.URL
	RegistrarServiceURL     *url.URL
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

var (
	Services       ServiceSettings
	SmartContracts SmartContractSettings
)

// Setting this env var will enable certain presets.
var NetworkEnvVar = "PIRI_NETWORK"

const DefaultNetwork = ForgeProd

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
		PrincipalMapping:        forgeProdPrincipalMapping,
		SigningServiceEndpoint:  forgeProdSigningServiceURL,
		RegistrarServiceURL:     forgeProdRegistrarServiceURL,
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

	warmStagingSigningServiceEndpoint := lo.Must(url.Parse("https://staging.signer.warm.storacha.network"))
	warmStagingRegistrarServiceURL := lo.Must(url.Parse("https://staging.registrar.warm.storacha.network"))

	return ServiceSettings{
		IPNIAnnounceURLs:        warmStagingIPNIAnnounceURLs,
		IndexingServiceURL:      warmStagingIndexingServiceURL,
		IndexingServiceDID:      warmStagingIndexingServiceDID,
		EgressTrackerServiceURL: warmStagingEgressTrackerServiceURL,
		EgressTrackerServiceDID: warmStagingEgressTrackerServiceDID,
		UploadServiceURL:        warmStagingUploadServiceURL,
		UploadServiceDID:        warmStagingUploadServiceDID,
		PrincipalMapping:        warmStagingPrincipalMapping,
		SigningServiceEndpoint:  warmStagingSigningServiceEndpoint,
		RegistrarServiceURL:     warmStagingRegistrarServiceURL,
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
		PrincipalMapping:        stagingPrincipalMapping,
		SigningServiceEndpoint:  nil,
		RegistrarServiceURL:     nil,
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
		PrincipalMapping:        prodPrincipalMapping,
		SigningServiceEndpoint:  nil,
		RegistrarServiceURL:     nil,
	}
}

// Contract settings for calibration network
var calibnetSettings = SmartContractSettings{
	// PDPVerifier contract address (see https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts)
	Verifier: common.HexToAddress("0xB020524bdE8926cD430A4F79B2AaccFd2694793b"),
	// This contract and its address are owned by storacha
	ProviderRegistry: common.HexToAddress("0x8D0560F93022414e7787207682a8D562de02D62f"),
	// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
	Service:     common.HexToAddress("0xB9753937D3Bc1416f7d741d75b1671A1edb3e10A"),
	ServiceView: common.HexToAddress("0xb2eC3e67753F1c05e8B318298Bd0eD89046a3031"),
	// Filecoin calibration chain ID
	ChainID: big.NewInt(314159),
	// PayerAddress is the Storacha Owned address that pays SPs
	PayerAddress: common.HexToAddress("0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD"),
}

// Contract settings for mainnet
// TODO (vic): These use calibnet addresses as placeholders until mainnet contracts are deployed
var mainnetSettings = SmartContractSettings{
	// PDPVerifier contract address (see https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts)
	Verifier: common.HexToAddress("0xB020524bdE8926cD430A4F79B2AaccFd2694793b"),
	// This contract and its address are owned by storacha
	ProviderRegistry: common.HexToAddress("0x8D0560F93022414e7787207682a8D562de02D62f"),
	// This contract and its address are owned by storacha, and uses ProviderRegistry for membership
	Service:     common.HexToAddress("0xB9753937D3Bc1416f7d741d75b1671A1edb3e10A"),
	ServiceView: common.HexToAddress("0xb2eC3e67753F1c05e8B318298Bd0eD89046a3031"),
	// Filecoin mainnet chain ID
	// TODO (vic): Change to 314 once mainnet contracts deployed
	ChainID: big.NewInt(314159),
	// PayerAddress is the Storacha Owned address that pays SPs
	// TODO (vic): Update with mainnet payer address
	PayerAddress: common.HexToAddress("0x8d3d7cE4F43607C9d0ac01f695c7A9caC135f9AD"),
}

// GetPreset returns the complete preset configuration for a given network
func GetPreset(network Network) Preset {
	switch network {
	case Prod:
		return Preset{
			Services:       prodServiceSettings(),
			SmartContracts: mainnetSettings,
		}
	case Staging:
		return Preset{
			Services:       stagingServiceSettings(),
			SmartContracts: calibnetSettings,
		}
	case WarmStaging:
		return Preset{
			Services:       warmStagingServiceSettings(),
			SmartContracts: calibnetSettings,
		}
	default: // ForgeProd
		return Preset{
			Services:       forgeProdServiceSettings(),
			SmartContracts: mainnetSettings,
		}
	}
}

func init() {
	network, err := ParseNetwork(os.Getenv(NetworkEnvVar))
	if err != nil {
		network = DefaultNetwork
	}

	preset := GetPreset(network)

	Services = preset.Services
	SmartContracts = preset.SmartContracts
}
