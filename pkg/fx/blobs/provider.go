package blobs

import (
	"fmt"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/principal"

	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/internal/digestutil"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/service/blobs"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

func NewService(
	cfg app.PiriConfig,
	id principal.Signer,
	blobStore blobstore.Blobstore,
	allocationStore allocationstore.AllocationStore,
) (*blobs.BlobService, error) {
	if cfg.PublicURL == nil {
		return nil, fmt.Errorf("public URL required for blob service")
	}

	if !id.DID().Defined() {
		return nil, fmt.Errorf("invalid DID for blob service")
	}

	accessURL := cfg.PublicURL
	accessURL.Path = "/blob"
	ap, err := access.NewPatternAccess(fmt.Sprintf("%s/{blob}", accessURL.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize access pattern for blob service: %w", err)
	}

	accessKeyID := id.DID().String()
	idDigest, _ := multihash.Sum(id.Encode(), multihash.SHA2_256, -1)
	secretAccessKey := digestutil.Format(idDigest)
	ps, err := presigner.NewS3RequestPresigner(accessKeyID, secretAccessKey, *cfg.PublicURL, "blob")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize presigner for blob service: %w", err)
	}

	return blobs.New(
		blobs.WithAccess(ap),
		blobs.WithPresigner(ps),
		blobs.WithBlobstore(blobStore),
		blobs.WithAllocationStore(allocationStore),
	)
}
