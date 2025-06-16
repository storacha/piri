package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/maurl"
	"github.com/multiformats/go-multiaddr"
	"github.com/storacha/go-libstoracha/ipnipublisher/store"
	"github.com/storacha/go-libstoracha/metadata"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ucanhttp "github.com/storacha/go-ucanto/transport/http"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/claimstore"
	"github.com/storacha/piri/pkg/store/delegationstore"
	"github.com/storacha/piri/pkg/store/receiptstore"
)

var log = logging.Logger("aws-providers")

// ProvideConfig creates AWS Config from environment variables
func ProvideConfig() (Config, error) {
	ctx := context.Background()
	cfg := FromEnv(ctx)
	return cfg, nil
}

// ProvideAWSBlobstore creates an S3-backed blobstore
func ProvideAWSBlobstore(cfg Config) blobstore.Blobstore {
	blobStoreOpts := cfg.S3Options
	if cfg.BlobStoreBucketAccessKeyID != "" && cfg.BlobStoreBucketSecretAccessKey != "" {
		blobStoreOpts = append(blobStoreOpts, func(opts *s3.Options) {
			opts.Region = cfg.BlobStoreBucketRegion
			opts.Credentials = credentials.NewStaticCredentialsProvider(
				cfg.BlobStoreBucketAccessKeyID,
				cfg.BlobStoreBucketSecretAccessKey,
				"",
			)
			if cfg.BlobStoreBucketEndpoint != "" {
				opts.BaseEndpoint = &cfg.BlobStoreBucketEndpoint
				opts.UsePathStyle = true
			}
		})
	}
	var formatKey KeyFormatterFunc
	if cfg.BlobStoreBucketKeyPattern != "" {
		formatKey = NewPatternKeyFormatter(cfg.BlobStoreBucketKeyPattern)
	}
	return NewS3BlobStore(cfg.Config, cfg.BlobStoreBucket, formatKey, blobStoreOpts...)
}

// ProvideAWSAllocationStore creates a DynamoDB-backed allocation store
func ProvideAWSAllocationStore(cfg Config) allocationstore.AllocationStore {
	return NewDynamoAllocationStore(cfg.Config, cfg.AllocationsTableName, cfg.DynamoOptions...)
}

// ProvideAWSClaimStore creates an S3-backed claim store
func ProvideAWSClaimStore(cfg Config) (claimstore.ClaimStore, error) {
	s3Store := NewS3Store(cfg.Config, cfg.ClaimStoreBucket, cfg.ClaimStorePrefix, cfg.S3Options...)
	return delegationstore.NewDelegationStore(s3Store)
}

// ProvideAWSPublisherStore creates the publisher store with S3 and DynamoDB backends
func ProvideAWSPublisherStore(cfg Config) store.PublisherStore {
	ipniStore := NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix, cfg.S3Options...)
	chunkLinksTable := NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName, cfg.DynamoOptions...)
	metadataTable := NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName, cfg.DynamoOptions...)
	return store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))
}

// ProvideAWSReceiptStore creates an S3-backed receipt store with DynamoDB index
func ProvideAWSReceiptStore(cfg Config) (receiptstore.ReceiptStore, error) {
	ranLinkIndex := NewDynamoRanLinkIndex(cfg.Config, cfg.RanLinkIndexTableName, cfg.DynamoOptions...)
	s3ReceiptStore := NewS3Store(cfg.Config, cfg.ReceiptStoreBucket, cfg.ReceiptStorePrefix, cfg.S3Options...)
	return receiptstore.NewReceiptStore(s3ReceiptStore, ranLinkIndex)
}

// ProvideAWSPublicURL parses and provides the public URL
func ProvideAWSPublicURL(cfg Config) (*url.URL, error) {
	return url.Parse(cfg.PublicURL)
}

// ProvideAWSBlobsPublicURL parses and provides the blobs public URL
func ProvideAWSBlobsPublicURL(cfg Config) (*url.URL, error) {
	return url.Parse(cfg.BlobsPublicURL)
}

// ProvideAWSIndexingServiceDID parses and provides the indexing service DID
func ProvideAWSIndexingServiceDID(cfg Config) (did.DID, error) {
	return did.Parse(cfg.IndexingServiceDID)
}

// ProvideAWSIndexingServiceURL parses and provides the indexing service URL
func ProvideAWSIndexingServiceURL(cfg Config) (*url.URL, error) {
	return url.Parse(cfg.IndexingServiceURL)
}

// ProvideAWSIndexingServiceProofs parses and provides the indexing service proofs
func ProvideAWSIndexingServiceProofs(cfg Config) (delegation.Proofs, error) {
	proof, err := delegation.Parse(cfg.IndexingServiceProof)
	if err != nil {
		return nil, fmt.Errorf("parsing indexing service proof: %w", err)
	}
	proofs := delegation.Proofs{delegation.FromDelegation(proof)}
	if len(proofs) == 0 {
		return nil, ErrIndexingServiceProofsMissing
	}
	return proofs, nil
}

// ProvideAWSPeerMultiAddress provides the multiaddr for the public URL
func ProvideAWSPeerMultiAddress(pubURL *url.URL) (multiaddr.Multiaddr, error) {
	return maurl.FromURL(pubURL)
}

// ProvideAWSBlobsAccess provides blob access configuration
func ProvideAWSBlobsAccess(cfg Config, blobsPublicURL *url.URL) (access.Access, error) {
	if cfg.BlobStoreBucketKeyPattern == "" {
		return nil, nil
	}

	pattern := blobsPublicURL.String()
	if strings.HasSuffix(pattern, "/") {
		pattern = fmt.Sprintf("%s%s", pattern, cfg.BlobStoreBucketKeyPattern)
	} else {
		pattern = fmt.Sprintf("%s/%s", pattern, cfg.BlobStoreBucketKeyPattern)
	}
	return access.NewPatternAccess(pattern)
}

// ProvideAWSBlobsPreSigner provides the presigner from the blobstore
func ProvideAWSBlobsPreSigner(blobStore blobstore.Blobstore) presigner.RequestPresigner {
	// Type assert to get the S3BlobStore which has PresignClient method
	s3BlobStore, ok := blobStore.(*S3BlobStore)
	if !ok {
		log.Warn("blobstore is not S3BlobStore, presigner will be nil")
		return nil
	}
	return s3BlobStore.PresignClient()
}

// ProvideAWSPDPService creates the PDP service if configured
func ProvideAWSPDPService(cfg Config) (pdp.PDP, error) {
	if cfg.SQSPDPPieceAggregatorURL == "" || cfg.CurioURL == "" {
		log.Info("PDP Service is disabled. Operating w/o PDP")
		return nil, nil
	}

	return NewPDP(cfg)
}

// ProvidePrincipalSigner extracts the principal signer from config
func ProvidePrincipalSigner(cfg Config) principal.Signer {
	return cfg.Signer
}

// ProvideIPNIAnnounceURLs provides the IPNI announce URLs
func ProvideIPNIAnnounceURLs(cfg Config) []url.URL {
	return cfg.IPNIAnnounceURLs
}

// ProvideServicePrincipalMapping provides the service principal mapping
func ProvideServicePrincipalMapping(cfg Config) map[string]string {
	return cfg.PrincipalMapping
}

// ProvideUploadServiceDID provides the upload service DID with defaults
func ProvideUploadServiceDID(cfg Config) did.DID {
	// The Config struct doesn't have UploadServiceDID, so use presets
	return presets.UploadServiceDID
}

// ProvideUploadServiceURL provides the upload service URL with defaults
func ProvideUploadServiceURL(cfg Config) *url.URL {
	// The Config struct doesn't have UploadServiceURL, so use presets
	return presets.UploadServiceURL
}

// ProvideUploadServiceConnection provides the upload service connection
// This is provided with name:"upload" to distinguish from indexing service connection
func ProvideUploadServiceConnection(uploadDID did.DID, uploadURL *url.URL) (client.Connection, error) {
	channel := ucanhttp.NewHTTPChannel(uploadURL)
	conn, err := client.NewConnection(uploadDID, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create upload connection: %w", err)
	}
	return conn, nil
}

// ProvideIndexingServiceConnection provides the indexing service connection
// This is provided with name:"indexing" to distinguish from upload service connection
func ProvideIndexingServiceConnection(indexingDID did.DID, indexingURL *url.URL) (client.Connection, error) {
	channel := ucanhttp.NewHTTPChannel(indexingURL)
	conn, err := client.NewConnection(indexingDID, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create indexing connection: %w", err)
	}
	return conn, nil
}
