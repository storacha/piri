package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	multihash "github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
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

// List implements allocationstore.AllocationStore.
func (d *DynamoAllocationStore) List(ctx context.Context, mh multihash.Multihash) ([]allocation.Allocation, error) {
	keyEx := expression.Key("hash").Equal(expression.Value(digestutil.Format(mh)))
	expr, err := expression.NewBuilder().WithKeyCondition(keyEx).Build()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var allocations []allocation.Allocation
	queryPaginator := dynamodb.NewQueryPaginator(d.dynamoDbClient, &dynamodb.QueryInput{
		TableName:                 aws.String(d.tableName),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ConsistentRead:            aws.Bool(true),
	})
	for queryPaginator.HasMorePages() {
		response, err := queryPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("querying allocations: %w", err)
		}
		var allocationPage []allocationItem
		err = attributevalue.UnmarshalListOfMaps(response.Items, &allocationPage)
		if err != nil {
			return nil, fmt.Errorf("parsing query responses: %w", err)
		}

		for _, item := range allocationPage {
			a, err := allocation.Decode(item.Allocation, dagcbor.Decode)
			if err != nil {
				return nil, fmt.Errorf("decoding data: %w", err)
			}
			allocations = append(allocations, a)
		}
	}
	return allocations, nil
}

// Put implements allocationstore.AllocationStore.
func (d *DynamoAllocationStore) Put(ctx context.Context, alloc allocation.Allocation) error {
	data, err := allocation.Encode(alloc, dagcbor.Encode)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}
	item, err := attributevalue.MarshalMap(allocationItem{
		Hash:       digestutil.Format(alloc.Blob.Digest),
		Cause:      alloc.Cause.String(),
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
	Cause      string `dynamodbav:"cause"`
	Allocation []byte `dynamodbav:"allocation"`
}

var _ allocationstore.AllocationStore = (*DynamoAllocationStore)(nil)
