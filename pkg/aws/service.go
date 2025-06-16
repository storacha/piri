package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/storacha/go-libstoracha/piece/piece"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	"github.com/storacha/go-ucanto/principal/signer"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/pdp"
	"github.com/storacha/piri/pkg/pdp/aggregator"
	"github.com/storacha/piri/pkg/pdp/curio"
	"github.com/storacha/piri/pkg/pdp/pieceadder"
	"github.com/storacha/piri/pkg/pdp/piecefinder"
	"github.com/storacha/piri/pkg/presets"
	"github.com/storacha/piri/pkg/services"
	"github.com/storacha/piri/pkg/services/types"
)

// ErrMissingSecret means that the value returned from Secrets was empty
var ErrMissingSecret = errors.New("missing value for secret")

func mustGetEnv(envVar string) string {
	value := os.Getenv(envVar)
	if len(value) == 0 {
		panic(fmt.Errorf("missing env var: %s", envVar))
	}
	return value
}

var ErrIndexingServiceProofsMissing = errors.New("indexing service proofs are missing")

type AWSAggregator struct {
	pieceAggregatorQueue *SQSPieceQueue
}

// AggregatePiece is the frontend to aggregation
func (aa *AWSAggregator) AggregatePiece(ctx context.Context, pieceLink piece.PieceLink) error {
	return aa.pieceAggregatorQueue.Queue(ctx, pieceLink)
}

type PDP struct {
	aggregator  *AWSAggregator
	pieceAdder  pieceadder.PieceAdder
	pieceFinder piecefinder.PieceFinder
}

// Aggregator implements pdp.PDP.
func (p *PDP) Aggregator() aggregator.Aggregator {
	return p.aggregator
}

// PieceAdder implements pdp.PDP.
func (p *PDP) PieceAdder() pieceadder.PieceAdder {
	return p.pieceAdder
}

// PieceFinder implements pdp.PDP.
func (p *PDP) PieceFinder() piecefinder.PieceFinder {
	return p.pieceFinder
}

func NewPDP(cfg Config) (*PDP, error) {
	curioURL, err := url.Parse(cfg.CurioURL)
	if err != nil {
		return nil, fmt.Errorf("parsing curio URL: %w", err)
	}
	curioAuth, err := curio.CreateCurioJWTAuthHeader("storacha", cfg.Signer)
	if err != nil {
		return nil, fmt.Errorf("generating curio JWT: %w", err)
	}
	curioClient := curio.New(http.DefaultClient, curioURL, curioAuth)
	return &PDP{
		aggregator: &AWSAggregator{
			pieceAggregatorQueue: NewSQSPieceQueue(cfg.Config, cfg.SQSPDPPieceAggregatorURL),
		},
		pieceAdder:  pieceadder.NewCurioAdder(curioClient),
		pieceFinder: piecefinder.NewCurioFinder(curioClient),
	}, nil
}

var _ pdp.PDP = (*PDP)(nil)

type Config struct {
	Config                         aws.Config
	S3Options                      []func(*s3.Options)
	DynamoOptions                  []func(*dynamodb.Options)
	SentryDSN                      string
	SentryEnvironment              string
	AllocationsTableName           string
	BlobStoreBucketEndpoint        string
	BlobStoreBucketRegion          string
	BlobStoreBucketAccessKeyID     string
	BlobStoreBucketSecretAccessKey string
	BlobStoreBucketKeyPattern      string
	BlobStoreBucket                string
	AggregatesBucket               string
	AggregatesPrefix               string
	BufferBucket                   string
	BufferPrefix                   string
	ChunkLinksTableName            string
	MetadataTableName              string
	IPNIStoreBucket                string
	IPNIStorePrefix                string
	IPNIAnnounceURLs               []url.URL
	ClaimStoreBucket               string
	ClaimStorePrefix               string
	PublicURL                      string
	IndexingServiceDID             string
	IndexingServiceURL             string
	IndexingServiceProof           string
	IPNIPublisherAnnounceAddress   string
	BlobsPublicURL                 string
	RanLinkIndexTableName          string
	ReceiptStoreBucket             string
	ReceiptStorePrefix             string
	SQSPDPPieceAggregatorURL       string
	SQSPDPAggregateSubmitterURL    string
	SQSPDPPieceAccepterURL         string
	PDPProofSet                    uint64
	CurioURL                       string
	PrincipalMapping               map[string]string
	principal.Signer
}

func mustGetSSMParams(ctx context.Context, client *ssm.Client, names ...string) map[string]string {
	response, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names:          names,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		panic(fmt.Errorf("retrieving SSM parameters: %w", err))
	}
	params := map[string]string{}
	for _, name := range names {
		value := ""
		for _, p := range response.Parameters {
			if *p.Name == name {
				value = *p.Value
				break
			}
		}
		if value == "" {
			panic(ErrMissingSecret)
		}
		params[name] = value
	}
	return params
}

// FromEnv constructs the AWS Configuration from the environment
func FromEnv(ctx context.Context) Config {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Errorf("loading aws default config: %w", err))
	}

	ssmClient := ssm.NewFromConfig(awsConfig)
	secretNames := []string{mustGetEnv("PRIVATE_KEY")}
	for _, n := range []string{
		"BLOB_STORE_BUCKET_ACCESS_KEY_ID",
		"BLOB_STORE_BUCKET_SECRET_ACCESS_KEY",
	} {
		if os.Getenv(n) != "" {
			secretNames = append(secretNames, os.Getenv(n))
		}
	}
	secrets := mustGetSSMParams(ctx, ssmClient, secretNames...)

	id, err := ed25519.Parse(secrets[mustGetEnv("PRIVATE_KEY")])
	if err != nil {
		panic(fmt.Errorf("parsing private key: %s", err))
	}

	if len(os.Getenv("DID")) != 0 {
		d, err := did.Parse(os.Getenv("DID"))
		if err != nil {
			panic(fmt.Errorf("parsing DID: %w", err))
		}
		id, err = signer.Wrap(id, d)
		if err != nil {
			panic(fmt.Errorf("wrapping server DID: %w", err))
		}
	}

	ipniStoreKeyPrefix := os.Getenv("IPNI_STORE_KEY_PREFIX")
	if len(ipniStoreKeyPrefix) == 0 {
		ipniStoreKeyPrefix = "ipni/v1/ad/"
	}

	ipniPublisherAnnounceAddress := fmt.Sprintf("/dns/%s/https", mustGetEnv("IPNI_STORE_BUCKET_REGIONAL_DOMAIN"))

	blobsPublicURL := "https://" + mustGetEnv("BLOB_STORE_BUCKET_REGIONAL_DOMAIN")
	proofSetString := os.Getenv("PDP_PROOFSET")
	var proofSet uint64
	if len(proofSetString) > 0 {
		proofSet, err = strconv.ParseUint(proofSetString, 10, 64)
		if err != nil {
			panic(fmt.Errorf("parsing pdp proofset: %w", err))
		}
	}

	var principalMapping map[string]string
	if os.Getenv("PRINCIPAL_MAPPING") != "" {
		principalMapping = map[string]string{}
		maps.Copy(principalMapping, presets.PrincipalMapping)
		var pm map[string]string
		err := json.Unmarshal([]byte(os.Getenv("PRINCIPAL_MAPPING")), &pm)
		if err != nil {
			panic(fmt.Errorf("parsing principal mapping: %w", err))
		}
		maps.Copy(principalMapping, pm)
	} else {
		principalMapping = presets.PrincipalMapping
	}

	var ipniAnnounceURLs []url.URL
	if os.Getenv("IPNI_ANNOUNCE_URLS") != "" {
		var urls []string
		err := json.Unmarshal([]byte(os.Getenv("IPNI_ANNOUNCE_URLS")), &urls)
		if err != nil {
			panic(fmt.Errorf("parsing IPNI announce URLs JSON: %w", err))
		}
		for _, s := range urls {
			url, err := url.Parse(s)
			if err != nil {
				panic(fmt.Errorf("parsing IPNI announce URL: %s: %w", s, err))
			}
			ipniAnnounceURLs = append(ipniAnnounceURLs, *url)
		}
	} else {
		ipniAnnounceURLs = presets.IPNIAnnounceURLs
	}

	return Config{
		Config:                         awsConfig,
		SentryDSN:                      os.Getenv("SENTRY_DSN"),
		SentryEnvironment:              os.Getenv("SENTRY_ENVIRONMENT"),
		Signer:                         id,
		ChunkLinksTableName:            mustGetEnv("CHUNK_LINKS_TABLE_NAME"),
		MetadataTableName:              mustGetEnv("METADATA_TABLE_NAME"),
		IPNIStoreBucket:                mustGetEnv("IPNI_STORE_BUCKET_NAME"),
		IPNIStorePrefix:                ipniStoreKeyPrefix,
		IPNIPublisherAnnounceAddress:   ipniPublisherAnnounceAddress,
		IPNIAnnounceURLs:               ipniAnnounceURLs,
		BlobsPublicURL:                 blobsPublicURL,
		ClaimStoreBucket:               mustGetEnv("CLAIM_STORE_BUCKET_NAME"),
		ClaimStorePrefix:               os.Getenv("CLAIM_STORE_KEY_REFIX"),
		AllocationsTableName:           mustGetEnv("ALLOCATIONS_TABLE_NAME"),
		BlobStoreBucketEndpoint:        os.Getenv("BLOB_STORE_BUCKET_ENDPOINT"),
		BlobStoreBucketRegion:          os.Getenv("BLOB_STORE_BUCKET_REGION"),
		BlobStoreBucketAccessKeyID:     secrets[os.Getenv("BLOB_STORE_BUCKET_ACCESS_KEY_ID")],
		BlobStoreBucketSecretAccessKey: secrets[os.Getenv("BLOB_STORE_BUCKET_SECRET_ACCESS_KEY")],
		BlobStoreBucketKeyPattern:      os.Getenv("BLOB_STORE_BUCKET_KEY_PATTERN"),
		BlobStoreBucket:                mustGetEnv("BLOB_STORE_BUCKET_NAME"),
		BufferBucket:                   os.Getenv("BUFFER_BUCKET_NAME"),
		BufferPrefix:                   os.Getenv("BUFFER_KEY_PREFIX"),
		AggregatesBucket:               os.Getenv("AGGREGATES_BUCKET_NAME"),
		AggregatesPrefix:               os.Getenv("AGGREGATES_KEY_PREFIX"),
		PublicURL:                      mustGetEnv("PUBLIC_URL"),
		IndexingServiceDID:             mustGetEnv("INDEXING_SERVICE_DID"),
		IndexingServiceURL:             mustGetEnv("INDEXING_SERVICE_URL"),
		IndexingServiceProof:           mustGetEnv("INDEXING_SERVICE_PROOF"),
		RanLinkIndexTableName:          mustGetEnv("RAN_LINK_INDEX_TABLE_NAME"),
		ReceiptStoreBucket:             mustGetEnv("RECEIPT_STORE_BUCKET_NAME"),
		ReceiptStorePrefix:             os.Getenv("RECEIPT_STORE_KEY_PREFIX"),
		SQSPDPPieceAggregatorURL:       os.Getenv("PIECE_AGGREGATOR_QUEUE_URL"),
		SQSPDPAggregateSubmitterURL:    os.Getenv("AGGREGATE_SUBMITTER_QUEUE_URL"),
		SQSPDPPieceAccepterURL:         os.Getenv("PIECE_ACCEPTER_QUEUE_URL"),
		PDPProofSet:                    proofSet,
		CurioURL:                       os.Getenv("CURIO_URL"),
		PrincipalMapping:               principalMapping,
	}
}

func Construct(cfg Config) (types.Service, error) {
	// Create a temporary fx app just to construct the service
	var service types.Service

	app := fx.New(
		// Supply the config
		fx.Supply(cfg),

		// Include AWS configuration module
		ConfigModule,

		// Include AWS datastores
		fx.Provide(
			ProvideAWSBlobstore,
			ProvideAWSAllocationStore,
			ProvideAWSClaimStore,
			ProvideAWSPublisherStore,
			ProvideAWSReceiptStore,
		),

		// Include service implementations
		services.ServiceModule,

		// Extract the service to return it
		fx.Invoke(func(s types.Service) {
			service = s
		}),

		// Don't start the lifecycle (we just want to construct the service)
		fx.NopLogger,
	)

	// Build the dependency graph without starting
	if err := app.Err(); err != nil {
		return nil, fmt.Errorf("constructing service with fx: %w", err)
	}

	return service, nil
}
