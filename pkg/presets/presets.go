package presets

import (
	"net/url"
	"os"

	"github.com/samber/lo"
	"github.com/storacha/go-ucanto/did"
)

var UCANServerPublicURL = lo.Must(url.Parse("https://localhost:3000"))
var (
	IPNIAnnounceURLs   []url.URL
	IndexingServiceDID did.DID
	IndexingServiceURL *url.URL
	UploadServiceURL   *url.URL
	UploadServiceDID   did.DID
	PrincipalMapping   map[string]string
)

// Setting this value to anything will cause production presets to be used, else use warm staging.
var ProductionPresetsEnvVar = "PIRI_PRODUCTION_PRESETS"

func init() {
	if os.Getenv(ProductionPresetsEnvVar) != "" {
		IPNIAnnounceURLs = prodIPNIAnnounceURLs
		IndexingServiceURL = prodIndexingServiceURL
		IndexingServiceDID = prodIndexingServiceDID
		UploadServiceURL = prodUploadServiceURL
		UploadServiceDID = prodUploadServiceDID
		PrincipalMapping = prodPrincipalMapping
	}
	IPNIAnnounceURLs = warmStageIPNIAnnounceURLs
	IndexingServiceURL = warmStageIndexingServiceURL
	IndexingServiceDID = warmStageIndexingServiceDID
	UploadServiceURL = warmStageUploadServiceURL
	UploadServiceDID = warmStageUploadServiceDID
	PrincipalMapping = warmStagePrincipalMapping

}

// Warm Staging preset values (default)
var (
	warmStageCIDContactIPNIAnnounceURL = lo.Must(url.Parse("https://cid.contact/announce"))
	warmStageStorachaIPNIAnnounceURLs  = lo.Must(url.Parse("https://staging.ipni.warm.storacha.network"))
	warmStageIPNIAnnounceURLs          = []url.URL{*warmStageCIDContactIPNIAnnounceURL, *warmStageStorachaIPNIAnnounceURLs}

	warmStageIndexingServiceURL = lo.Must(url.Parse("https://staging.indexer.warm.storacha.network"))
	warmStageIndexingServiceDID = lo.Must(did.Parse("did:web:staging.indexer.warm.storacha.network"))

	warmStageUploadServiceURL = lo.Must(url.Parse("https://staging.up.warm.storacha.network"))
	warmStageUploadServiceDID = lo.Must(did.Parse("did:web:staging.up.warm.storacha.network"))

	warmStagePrincipalMapping = map[string]string{
		warmStageUploadServiceDID.String(): "did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v"}
)

// Production preset values
var (
	prodCIDContactIPNIAnnounceURL = lo.Must(url.Parse("https://cid.contact/announce"))
	prodStorachaIPNIAnnounceURL   = lo.Must(url.Parse("https://ipni.storacha.network"))
	prodIPNIAnnounceURLs          = []url.URL{*prodCIDContactIPNIAnnounceURL, *prodStorachaIPNIAnnounceURL}

	prodIndexingServiceURL = lo.Must(url.Parse("https://indexer.storacha.network"))
	prodIndexingServiceDID = lo.Must(did.Parse("did:web:indexer.storacha.network"))

	prodUploadServiceURL = lo.Must(url.Parse("https://up.storacha.network"))
	prodUploadServiceDID = lo.Must(did.Parse("did:web:up.storacha.network"))

	prodPrincipalMapping = map[string]string{
		"did:web:staging.up.storacha.network": "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		"did:web:up.storacha.network":         "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
		"did:web:staging.web3.storage":        "did:key:z6MkhcbEpJpEvNVDd3n5RurquVdqs5dPU16JDU5VZTDtFgnn",
		"did:web:web3.storage":                "did:key:z6MkqdncRZ1wj8zxCTDUQ8CRT8NQWd63T7mZRvZUX8B7XDFi",
	}
)
