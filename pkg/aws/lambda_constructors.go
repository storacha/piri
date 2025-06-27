package aws

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"

	blobhttp "github.com/storacha/piri/pkg/services/blob/http"
	claimhttp "github.com/storacha/piri/pkg/services/claim/http"
	"github.com/storacha/piri/pkg/store/blobstore"
)

// ConstructBlobPutHandler creates just the dependencies needed for PUT /blob/:blob
func ConstructBlobPutHandler(cfg Config) (http.Handler, error) {
	var handler *blobhttp.Blob

	app := fx.New(
		fx.Supply(cfg),

		// Only provide what's needed for blob PUT
		fx.Provide(
			ProvideAWSBlobstore,       // S3 blob storage
			ProvideAWSAllocationStore, // DynamoDB allocations
			ProvideAWSBlobsPreSigner,  // URL presigner
		),

		// Construct the handler directly
		fx.Invoke(func(params blobhttp.Params) {
			handler = blobhttp.NewBlob(params)
		}),

		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		return nil, fmt.Errorf("constructing blob PUT handler: %w", err)
	}

	// Create minimal Echo instance with just this route
	e := echo.New()
	e.PUT("/blob/:blob", handler.PutBlob)

	return e, nil
}

// ConstructBlobGetHandler creates just the dependencies needed for GET /blob/:blob
func ConstructBlobGetHandler(cfg Config) (http.Handler, error) {
	var handler *blobhttp.Blob

	app := fx.New(
		fx.Supply(cfg),

		// Only provide what's needed for blob GET
		fx.Provide(
			ProvideAWSBlobstore, // S3 blob storage
			// Note: GET doesn't need PreSigner or AllocationStore
		),

		// Construct the handler directly
		fx.Invoke(func(blobStore blobstore.Blobstore) {
			handler = &blobhttp.Blob{
				BlobStore: blobStore,
			}
		}),

		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		return nil, fmt.Errorf("constructing blob GET handler: %w", err)
	}

	// Create minimal Echo instance with just this route
	e := echo.New()
	e.GET("/blob/:blob", handler.GetBlob)

	return e, nil
}

// ConstructClaimGetHandler creates just the dependencies needed for GET /claim/:claim
func ConstructClaimGetHandler(cfg Config) (http.Handler, error) {
	var handler *claimhttp.Claim

	app := fx.New(
		fx.Supply(cfg),

		// Only provide what's needed for claim GET
		fx.Provide(
			ProvideAWSClaimStore, // S3 claim storage
		),

		// Construct the handler using the constructor
		fx.Invoke(func(params claimhttp.Params) {
			handler = claimhttp.NewClaimHandler(params)
		}),

		fx.NopLogger,
	)

	if err := app.Err(); err != nil {
		return nil, fmt.Errorf("constructing claim GET handler: %w", err)
	}

	// Create minimal Echo instance with just this route
	e := echo.New()
	e.GET("/claim/:claim", handler.GetClaim)

	return e, nil
}
