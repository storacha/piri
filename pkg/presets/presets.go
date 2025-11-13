package presets

import (
	"net/url"
	"os"

	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/did"
)

var (
	IPNIAnnounceURLs        []url.URL
	IndexingServiceDID      did.DID
	IndexingServiceURL      *url.URL
	EgressTrackerServiceDID did.DID
	EgressTrackerServiceURL *url.URL
	UploadServiceURL        *url.URL
	UploadServiceDID        did.DID
	PrincipalMapping        map[string]string
	SigningServiceEndpoint  *url.URL
)

var (
	// PDPRecordKeeperAddress is the address of the PDP Service contract: https://github.com/FilOzone/pdp/?tab=readme-ov-file#v110
	// NB(forrest): for now we hardcode to the address to the calibnet service contract address
	// later this may be configured to the mainnet contract address for production settings.
	PDPRecordKeeperAddress = "0x6170dE2b09b404776197485F3dc6c968Ef948505"
)

// Setting this env var will enable certain presets.
var PresetsEnvVar = "PIRI_PRESETS"

// URL of the original and best IPNI node cid.contact.
var defaultIPNIAnnounceURL = lo.Must(url.Parse("https://cid.contact/announce"))

func init() {
	switch os.Getenv(PresetsEnvVar) {
	case "prod":
		IPNIAnnounceURLs = prodIPNIAnnounceURLs
		IndexingServiceURL = prodIndexingServiceURL
		IndexingServiceDID = prodIndexingServiceDID
		EgressTrackerServiceURL = prodEgressTrackerServiceURL
		EgressTrackerServiceDID = prodEgressTrackerServiceDID
		UploadServiceURL = prodUploadServiceURL
		UploadServiceDID = prodUploadServiceDID
		PrincipalMapping = prodPrincipalMapping
	case "staging":
		IPNIAnnounceURLs = stagingIPNIAnnounceURLs
		IndexingServiceURL = stagingIndexingServiceURL
		IndexingServiceDID = stagingIndexingServiceDID
		EgressTrackerServiceURL = stagingEgressTrackerServiceURL
		EgressTrackerServiceDID = stagingEgressTrackerServiceDID
		UploadServiceURL = stagingUploadServiceURL
		UploadServiceDID = stagingUploadServiceDID
		PrincipalMapping = stagingPrincipalMapping
	case "warm-staging":
		IPNIAnnounceURLs = warmStageIPNIAnnounceURLs
		IndexingServiceURL = warmStageIndexingServiceURL
		IndexingServiceDID = warmStageIndexingServiceDID
		EgressTrackerServiceURL = warmStageEgressTrackerServiceURL
		EgressTrackerServiceDID = warmStageEgressTrackerServiceDID
		UploadServiceURL = warmStageUploadServiceURL
		UploadServiceDID = warmStageUploadServiceDID
		PrincipalMapping = warmStagePrincipalMapping
		SigningServiceEndpoint = warmStageSigningServiceURL
	default:
		IPNIAnnounceURLs = forgeProdIPNIAnnounceURLs
		IndexingServiceURL = forgeProdIndexingServiceURL
		IndexingServiceDID = forgeProdIndexingServiceDID
		EgressTrackerServiceURL = forgeProdEgressTrackerServiceURL
		EgressTrackerServiceDID = forgeProdEgressTrackerServiceDID
		UploadServiceURL = forgeProdUploadServiceURL
		UploadServiceDID = forgeProdUploadServiceDID
		PrincipalMapping = forgeProdPrincipalMapping
		SigningServiceEndpoint = forgeProdSigningServiceURL
	}
}

// Forge Production preset values (default)
var (
	forgeProdStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://ipni.forge.storacha.network"))
	forgeProdIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *forgeProdStorachaIPNIAnnounceURL}

	forgeProdIndexingServiceURL = lo.Must(url.Parse("https://indexer.forge.storacha.network/claims"))
	forgeProdIndexingServiceDID = lo.Must(did.Parse("did:web:indexer.forge.storacha.network"))

	forgeProdEgressTrackerServiceURL = lo.Must(url.Parse("https://etracker.forge.storacha.network"))
	forgeProdEgressTrackerServiceDID = lo.Must(did.Parse("did:web:etracker.forge.storacha.network"))

	forgeProdUploadServiceURL = lo.Must(url.Parse("https://up.forge.storacha.network"))
	forgeProdUploadServiceDID = lo.Must(did.Parse("did:web:up.forge.storacha.network"))

	forgeProdPrincipalMapping = map[string]string{
		forgeProdUploadServiceDID.String():        "did:key:z6MkgSttS3n3R56yGX2Eufvbwc58fphomhAsLoBCZpZJzQbr",
		forgeProdIndexingServiceDID.String():      "did:key:z6Mkj8WmJQRy5jEnFN97uuc2qsjFdsYCuD5wE384Z1AMCFN7",
		forgeProdEgressTrackerServiceDID.String(): "did:key:z6MkuGS213fJGP7qGRG8Pn9mffCDU2vXgnSRY4JL1sumgpFX",
	}

	forgeProdSigningServiceURL = lo.Must(url.Parse("https://signer.forge.storacha.network"))
)

// Warm Staging preset values
var (
	warmStageStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://staging.ipni.warm.storacha.network"))
	warmStageIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *warmStageStorachaIPNIAnnounceURL}

	warmStageIndexingServiceURL = lo.Must(url.Parse("https://staging.indexer.warm.storacha.network/claims"))
	warmStageIndexingServiceDID = lo.Must(did.Parse("did:web:staging.indexer.warm.storacha.network"))

	warmStageEgressTrackerServiceURL = lo.Must(url.Parse("https://staging.etracker.warm.storacha.network"))
	warmStageEgressTrackerServiceDID = lo.Must(did.Parse("did:web:staging.etracker.warm.storacha.network"))

	warmStageUploadServiceURL = lo.Must(url.Parse("https://staging.up.warm.storacha.network"))
	warmStageUploadServiceDID = lo.Must(did.Parse("did:web:staging.up.warm.storacha.network"))

	warmStagePrincipalMapping = map[string]string{
		warmStageUploadServiceDID.String():        "did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v",
		warmStageIndexingServiceDID.String():      "did:key:z6Mkr4QkdinnXQmJ9JdnzwhcEjR8nMnuVPEwREyh9jp2Pb7k",
		warmStageEgressTrackerServiceDID.String(): "did:key:z6Mkqv9fjGQpNKQdgUxkq2VYH2nKiKZiGPxbtYjhJBz8wfAn",
	}

	warmStageSigningServiceURL = lo.Must(url.Parse("https://staging.signer.warm.storacha.network"))
)

// Staging preset values
var (
	stagingStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://staging.ipni.storacha.network"))
	stagingIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *stagingStorachaIPNIAnnounceURL}

	stagingIndexingServiceURL = lo.Must(url.Parse("https://staging.indexer.storacha.network/claims"))
	stagingIndexingServiceDID = lo.Must(did.Parse("did:web:staging.indexer.storacha.network"))

	stagingEgressTrackerServiceURL = lo.Must(url.Parse("https://staging.etracker.storacha.network"))
	stagingEgressTrackerServiceDID = lo.Must(did.Parse("did:web:staging.etracker.storacha.network"))

	stagingUploadServiceURL = lo.Must(url.Parse("https://staging.up.storacha.network"))
	stagingUploadServiceDID = lo.Must(did.Parse("did:web:staging.up.storacha.network"))

	stagingPrincipalMapping = map[string]string{
		stagingUploadServiceDID.String():        "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		stagingIndexingServiceDID.String():      "did:key:z6MkszJLAZ1tCHUTfDMKj9srMYA9zzLiPeMXijvmeECUScTZ",
		stagingEgressTrackerServiceDID.String(): "did:key:z6MkmSQ8ZZQffBaQo5mr3fArRsDxKPXyPQFroiB1H9EbAHod",
	}
)

// Production preset values
var (
	prodStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://ipni.storacha.network"))
	prodIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *prodStorachaIPNIAnnounceURL}

	prodIndexingServiceURL = lo.Must(url.Parse("https://indexer.storacha.network/claims"))
	prodIndexingServiceDID = lo.Must(did.Parse("did:web:indexer.storacha.network"))

	prodEgressTrackerServiceURL = lo.Must(url.Parse("https://etracker.storacha.network"))
	prodEgressTrackerServiceDID = lo.Must(did.Parse("did:web:etracker.storacha.network"))

	prodUploadServiceURL = lo.Must(url.Parse("https://up.storacha.network"))
	prodUploadServiceDID = lo.Must(did.Parse("did:web:up.storacha.network"))

	prodPrincipalMapping = map[string]string{
		prodUploadServiceDID.String():        "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
		prodIndexingServiceDID.String():      "did:key:z6MkqMSJxrjzvpqmP3kZhk7eCasBK6DX1jaVaG7wD72LYRm7",
		prodEgressTrackerServiceDID.String(): "did:key:z6MkiVMkL8MSzqi3iqFj2AjofQfL8wH6p7AcB2w34mKbyWfF",
	}
)
