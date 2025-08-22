package store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/delegator/internal/config"
)

var log = logging.Logger("store/dynamo")

// DynamoDB provides storage via AWS DynamoDB
type DynamoDB struct {
	db                    *dynamodb.Client
	initialized           bool
	allowListTableName    string
	providerInfoTableName string
	providerWeight        uint
}

// NewDynamoDBStore creates a new DynamoDB-backed store
func NewDynamoDBStore(config config.DynamoConfig) (*DynamoDB, error) {
	ctx := context.Background()

	// Create custom config resolver if endpoint is specified
	var opts []func(*awsconfig.LoadOptions) error
	if config.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: config.Endpoint,
			}, nil
		})
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))

		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "dummy",
				SecretAccessKey: "dummy",
			},
		}))
	}

	// Add region if specified
	if config.Region != "" {
		opts = append(opts, awsconfig.WithRegion(config.Region))
	}

	// Load AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Create store
	store := &DynamoDB{
		db:                    client,
		initialized:           false,
		allowListTableName:    config.AllowListTableName,
		providerInfoTableName: config.ProviderInfoTableName,
		providerWeight:        config.ProviderWeight,
	}

	return store, store.initialize(ctx, config)
}

// Initialize creates tables if they don't exist
func (d *DynamoDB) initialize(ctx context.Context, cfg config.DynamoConfig) error {
	if d.initialized {
		return nil
	}

	tables := []struct {
		name       string
		keySchema  []types.KeySchemaElement
		attributes []types.AttributeDefinition
		indexes    []types.GlobalSecondaryIndex
	}{
		{
			name: cfg.AllowListTableName,
			keySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String("did"),
					KeyType:       types.KeyTypeHash,
				},
			},
			attributes: []types.AttributeDefinition{
				{
					AttributeName: aws.String("did"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},
		},
		{
			name: cfg.ProviderInfoTableName,
			keySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String("provider"),
					KeyType:       types.KeyTypeHash,
				},
			},
			attributes: []types.AttributeDefinition{
				{
					AttributeName: aws.String("provider"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},
			// Note: DynamoDB automatically creates attributes as needed when items are put in the table
			// Additional fields for StorageProviderInfo:
			// - provider (string, primary key) - already defined above
			// - endpoint (string)
			// - address (string)
			// - proofSet (number)
			// - operatorEmail (string)
			// - proof (string)
			// - insertedAt (timestamp)
			// - updatedAt (timestamp)
		},
	}

	for _, table := range tables {
		// Check if table exists first
		_, err := d.db.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(table.name),
		})

		if err == nil {
			log.Infow("Table already exists", "table_name", table.name)
			continue
		}

		// Create table
		input := &dynamodb.CreateTableInput{
			TableName:            aws.String(table.name),
			KeySchema:            table.keySchema,
			AttributeDefinitions: table.attributes,
			BillingMode:          types.BillingModePayPerRequest, // Simpler than provisioned
		}

		_, err = d.db.CreateTable(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.name, err)
		}
	}

	d.initialized = true
	log.Infow("DynamoDB store initialized",
		"region", d.db.Options().Region,
		"endpoint", d.db.Options().EndpointResolver)
	allowed, err := did.Parse("did:key:z6MktKFLaLfEEisRzdLjRQXwzAMrfdvgYF8rBsgotdhTXTri")
	if err != nil {
		panic(err)
	}
	if err := d.AddAllowedDID(ctx, allowed); err != nil {
		panic(err)
	}
	return nil
}

// IsAllowedDID checks if a DID is allowed for onboarding (implements PersistentStore interface)
func (d *DynamoDB) IsAllowedDID(ctx context.Context, id did.DID) (bool, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.allowListTableName),
		Key: map[string]types.AttributeValue{
			"did": &types.AttributeValueMemberS{Value: id.String()},
		},
		ProjectionExpression: aws.String("did"), // Only retrieve the key
	}

	result, err := d.db.GetItem(ctx, input)
	if err != nil {
		return false, fmt.Errorf("failed to check DID %s allowlist: %w", id.String(), err)
	}

	return len(result.Item) > 0, nil
}

// AddAllowedDID adds a DID to the allowlist (implements PersistentStore interface)
func (d *DynamoDB) AddAllowedDID(ctx context.Context, id did.DID) error {
	// Use a simple approach - just add the required key directly
	// This avoids any serialization issues with the struct
	item := map[string]types.AttributeValue{
		"did":     &types.AttributeValueMemberS{Value: id.String()},
		"addedBy": &types.AttributeValueMemberS{Value: "system"},
		"addedAt": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"notes":   &types.AttributeValueMemberS{Value: "Added via API"},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.allowListTableName),
		Item:      item,
	}

	_, err := d.db.PutItem(ctx, input)
	if err != nil {
		log.Errorw("Error adding DID to allowlist", "did", id.String(), "error", err)
		return fmt.Errorf("failed to add DID to allowlist: %w", err)
	}

	return nil
}

// IsRegisteredDID checks if a DID is registered as a provider (implements PersistentStore interface)
func (d *DynamoDB) IsRegisteredDID(ctx context.Context, id did.DID) (bool, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.providerInfoTableName),
		Key: map[string]types.AttributeValue{
			"provider": &types.AttributeValueMemberS{Value: id.String()},
		},
		ProjectionExpression: aws.String("provider"), // Only retrieve the key
	}

	result, err := d.db.GetItem(ctx, input)
	if err != nil {
		return false, fmt.Errorf("failed to check provider registration: %w", err)
	}

	return len(result.Item) > 0, nil
}

// RegisterProvider registers a new provider (implements PersistentStore interface)
func (d *DynamoDB) RegisterProvider(ctx context.Context, info StorageProviderInfo) error {
	// Set timestamps if they're not already set
	now := time.Now()
	info.InsertedAt = now
	info.UpdatedAt = now

	// Log info for debugging
	log.Infow("Registering provider",
		"did", info.Provider,
		"endpoint", info.Endpoint)

	// Manually create the item map to ensure all required fields are properly set
	item := map[string]types.AttributeValue{
		"provider": &types.AttributeValueMemberS{Value: info.Provider},
	}

	// Add other fields with proper type conversion
	if info.Endpoint != "" {
		item["endpoint"] = &types.AttributeValueMemberS{Value: info.Endpoint}
	}

	if info.Address != "" {
		item["address"] = &types.AttributeValueMemberS{Value: info.Address}
	}

	// Use string representation for numeric values
	item["proofSet"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", info.ProofSet)}
	item["weight"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", d.providerWeight)}

	if info.OperatorEmail != "" {
		item["operatorEmail"] = &types.AttributeValueMemberS{Value: info.OperatorEmail}
	}

	if info.Proof != "" {
		item["proof"] = &types.AttributeValueMemberS{Value: info.Proof}
	}

	// Format timestamps as strings
	item["insertedAt"] = &types.AttributeValueMemberS{Value: info.InsertedAt.Format(time.RFC3339)}
	item["updatedAt"] = &types.AttributeValueMemberS{Value: info.UpdatedAt.Format(time.RFC3339)}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.providerInfoTableName),
		Item:      item,
	}

	_, err := d.db.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}

	return nil
}
