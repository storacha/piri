package presigner

import (
	"fmt"

	"github.com/multiformats/go-multihash"
	"github.com/storacha/go-ucanto/principal"
	"go.uber.org/fx"

	"github.com/storacha/go-libstoracha/digestutil"
	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/presigner"
)

var Module = fx.Module("presigner",
	fx.Provide(
		NewRequestPresigner,
	),
)

// NewRequestPresigner creates a new S3 request presigner
func NewRequestPresigner(cfg app.AppConfig, id principal.Signer) (presigner.RequestPresigner, error) {
	if cfg.Server.PublicURL.Scheme == "" {
		return nil, fmt.Errorf("public URL required for presigner")
	}

	accessKeyID := id.DID().String()
	idDigest, _ := multihash.Sum(id.Encode(), multihash.SHA2_256, -1)
	secretAccessKey := digestutil.Format(idDigest)

	return presigner.NewS3RequestPresigner(accessKeyID, secretAccessKey, cfg.Server.PublicURL, "blob")
}
