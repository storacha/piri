package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/piri/pkg/store"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/allocationstore/allocation"
)

// DynamoAllocationStore implements the AllocationStore interface on dynamodb
type DynamoAllocationStore struct {
	tableName      string
	dynamoDbClient *dynamodb.Client
}

// NewDynamoAllocationStore returns an AllocationStore connected to a AWS DynamoDB table
func NewDynamoAllocationStore(cfg aws.Config, tableName string, opts ...func(*dynamodb.Options)) *DynamoAllocationStore {
	return &DynamoAllocationStore{
		tableName:      tableName,
		dynamoDbClient: dynamodb.NewFromConfig(cfg, opts...),
	}
}

func (d *DynamoAllocationStore) Get(ctx context.Context, mh multihash.Multihash, space did.DID) (allocation.Allocation, error) {
	res, err := d.dynamoDbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"hash":  &types.AttributeValueMemberS{Value: digestutil.Format(mh)},
			"cause": &types.AttributeValueMemberS{Value: space.String()},
		},
	})
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("getting item: %w", err)
	}
	if res.Item == nil {
		return allocation.Allocation{}, store.ErrNotFound
	}
	var item allocationItem
	err = attributevalue.UnmarshalMap(res.Item, &item)
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("unmarshalling allocation item: %w", err)
	}
	alloc, err := allocation.Decode(item.Allocation, dagcbor.Decode)
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("decoding allocation: %w", err)
	}
	return alloc, nil
}

// GetAny retrieves any allocation for a blob (digest), regardless of space.
func (d *DynamoAllocationStore) GetAny(ctx context.Context, mh multihash.Multihash) (allocation.Allocation, error) {
	keyEx := expression.Key("hash").Equal(expression.Value(digestutil.Format(mh)))
	expr, err := expression.NewBuilder().WithKeyCondition(keyEx).Build()
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("building query: %w", err)
	}

	res, err := d.dynamoDbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(d.tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ConsistentRead:            aws.Bool(true),
		Limit:                     aws.Int32(1),
	})
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("querying allocation: %w", err)
	}
	if len(res.Items) == 0 {
		return allocation.Allocation{}, store.ErrNotFound
	}

	var item allocationItem
	err = attributevalue.UnmarshalMap(res.Items[0], &item)
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("unmarshalling allocation item: %w", err)
	}
	alloc, err := allocation.Decode(item.Allocation, dagcbor.Decode)
	if err != nil {
		return allocation.Allocation{}, fmt.Errorf("decoding allocation: %w", err)
	}
	return alloc, nil
}

// Exists checks if any allocation exists for a blob (digest).
func (d *DynamoAllocationStore) Exists(ctx context.Context, mh multihash.Multihash) (bool, error) {
	keyEx := expression.Key("hash").Equal(expression.Value(digestutil.Format(mh)))
	proj := expression.NamesList(expression.Name("hash"))
	expr, err := expression.NewBuilder().WithKeyCondition(keyEx).WithProjection(proj).Build()
	if err != nil {
		return false, fmt.Errorf("building query: %w", err)
	}

	res, err := d.dynamoDbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(d.tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ConsistentRead:            aws.Bool(true),
		Limit:                     aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("querying allocation: %w", err)
	}
	return len(res.Items) > 0, nil
}

// Put implements allocationstore.AllocationStore.
func (d *DynamoAllocationStore) Put(ctx context.Context, alloc allocation.Allocation) error {
	data, err := allocation.Encode(alloc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}
	item, err := attributevalue.MarshalMap(allocationItem{
		Hash:       digestutil.Format(alloc.Blob.Digest),
		Cause:      alloc.Space.String(),
		Allocation: data,
	})
	if err != nil {
		return fmt.Errorf("serializing item: %w", err)
	}
	_, err = d.dynamoDbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName), Item: item,
	})
	if err != nil {
		return fmt.Errorf("storing item: %w", err)
	}
	return nil
}

type allocationItem struct {
	Hash       string `dynamodbav:"hash"`
	Cause      string `dynamodbav:"cause"` // note: now space DID not invocation CID
	Allocation []byte `dynamodbav:"allocation"`
}

var _ allocationstore.AllocationStore = (*DynamoAllocationStore)(nil)
