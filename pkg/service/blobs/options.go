package blobs

import (
	"fmt"
	"net/url"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/piri/pkg/access"
	"github.com/storacha/piri/pkg/presigner"
	"github.com/storacha/piri/pkg/store/allocationstore"
	"github.com/storacha/piri/pkg/store/blobstore"
)

type options struct {
	access     access.Access
	allocStore allocationstore.AllocationStore
	blobStore  blobstore.Blobstore
	presigner  presigner.RequestPresigner
}

type Option func(*options) error

// WithLogLevel changes the log level for the claims subsystem.
func WithLogLevel(level string) Option {
	return func(o *options) error {
		logging.SetLogLevel("blobs", level)
		return nil
	}
}

func WithBlobstore(bs blobstore.Blobstore) Option {
	return func(o *options) error {
		o.blobStore = bs
		return nil
	}
}

func WithAccess(access access.Access) Option {
	return func(o *options) error {
		o.access = access
		return nil
	}
}

func WithPublicURLAccess(publicURL url.URL) Option {
	return func(o *options) error {
		accessURL := publicURL
		accessURL.Path = "/blob"
		access, err := access.NewPatternAccess(fmt.Sprintf("%s/{blob}", accessURL.String()))
		if err != nil {
			return err
		}
		o.access = access
		return nil
	}
}

func WithPresigner(presigner presigner.RequestPresigner) Option {
	return func(o *options) error {
		o.presigner = presigner
		return nil
	}
}

func WithPublicURLPresigner(id principal.Signer, publicURL url.URL) Option {
	return func(o *options) error {

		accessKeyID := id.DID().String()
		idDigest, _ := multihash.Sum(id.Encode(), multihash.SHA2_256, -1)
		secretAccessKey := digestutil.Format(idDigest)
		presigner, err := presigner.NewS3RequestPresigner(accessKeyID, secretAccessKey, publicURL, "blob")
		if err != nil {
			return err
		}
		o.presigner = presigner
		return nil
	}
}
func WithAllocationStore(allocationStore allocationstore.AllocationStore) Option {
	return func(o *options) error {
		o.allocStore = allocationStore
		return nil
	}
}

func WithDSAllocationStore(allocsDatastore datastore.Datastore) Option {
	return func(o *options) error {
		allocStore, err := allocationstore.NewDsAllocationStore(allocsDatastore)
		if err != nil {
			return err
		}
		o.allocStore = allocStore
		return nil
	}
}
