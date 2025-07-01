package presets

import (
	"net/url"
	"os"

	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/did"
)

var (
	IPNIAnnounceURLs   []url.URL
	IndexingServiceDID did.DID
	IndexingServiceURL *url.URL
	UploadServiceURL   *url.URL
	UploadServiceDID   did.DID
	PrincipalMapping   map[string]string
)

// Setting this env var will enable certain presets.
var PresetsEnvVar = "PIRI_PRESETS"

// URL of the original and best IPNI node cid.contact.
var defaultIPNIAnnounceURL = lo.Must(url.Parse("https://cid.contact/announce"))

func init() {
	if os.Getenv(PresetsEnvVar) != "prod" {
		IPNIAnnounceURLs = prodIPNIAnnounceURLs
		IndexingServiceURL = prodIndexingServiceURL
		IndexingServiceDID = prodIndexingServiceDID
		UploadServiceURL = prodUploadServiceURL
		UploadServiceDID = prodUploadServiceDID
		PrincipalMapping = prodPrincipalMapping
	} else if os.Getenv(PresetsEnvVar) == "staging" {
		IPNIAnnounceURLs = stagingIPNIAnnounceURLs
		IndexingServiceURL = stagingIndexingServiceURL
		IndexingServiceDID = stagingIndexingServiceDID
		UploadServiceURL = stagingUploadServiceURL
		UploadServiceDID = stagingUploadServiceDID
		PrincipalMapping = stagingPrincipalMapping
	} else {
		IPNIAnnounceURLs = warmStageIPNIAnnounceURLs
		IndexingServiceURL = warmStageIndexingServiceURL
		IndexingServiceDID = warmStageIndexingServiceDID
		UploadServiceURL = warmStageUploadServiceURL
		UploadServiceDID = warmStageUploadServiceDID
		PrincipalMapping = warmStagePrincipalMapping
	}
}

// Warm Staging preset values (default)
var (
	warmStageStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://staging.ipni.warm.storacha.network"))
	warmStageIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *warmStageStorachaIPNIAnnounceURL}

	warmStageIndexingServiceURL = lo.Must(url.Parse("https://staging.indexer.warm.storacha.network"))
	warmStageIndexingServiceDID = lo.Must(did.Parse("did:web:staging.indexer.warm.storacha.network"))

	warmStageUploadServiceURL = lo.Must(url.Parse("https://staging.up.warm.storacha.network"))
	warmStageUploadServiceDID = lo.Must(did.Parse("did:web:staging.up.warm.storacha.network"))

	warmStagePrincipalMapping = map[string]string{
		warmStageUploadServiceDID.String(): "did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v",
	}
)

// Staging preset values
var (
	stagingStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://staging.ipni.storacha.network"))
	stagingIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *stagingStorachaIPNIAnnounceURL}

	stagingIndexingServiceURL = lo.Must(url.Parse("https://staging.indexer.storacha.network"))
	stagingIndexingServiceDID = lo.Must(did.Parse("did:web:staging.indexer.storacha.network"))

	stagingUploadServiceURL = lo.Must(url.Parse("https://staging.up.storacha.network"))
	stagingUploadServiceDID = lo.Must(did.Parse("did:web:staging.up.storacha.network"))

	stagingPrincipalMapping = map[string]string{
		stagingUploadServiceDID.String(): "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
	}
)

// Production preset values
var (
	prodStorachaIPNIAnnounceURL = lo.Must(url.Parse("https://ipni.storacha.network"))
	prodIPNIAnnounceURLs        = []url.URL{*defaultIPNIAnnounceURL, *prodStorachaIPNIAnnounceURL}

	prodIndexingServiceURL = lo.Must(url.Parse("https://indexer.storacha.network"))
	prodIndexingServiceDID = lo.Must(did.Parse("did:web:indexer.storacha.network"))

	prodUploadServiceURL = lo.Must(url.Parse("https://up.storacha.network"))
	prodUploadServiceDID = lo.Must(did.Parse("did:web:up.storacha.network"))

	prodPrincipalMapping = map[string]string{
		prodUploadServiceDID.String(): "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
	}
)
