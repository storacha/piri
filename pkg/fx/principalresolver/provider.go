package principalresolver

import (
	"fmt"
	"strings"
	"time"

	"github.com/storacha/go-ucanto/did"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrievalserver "github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/validator"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/principalresolver"
)

var Module = fx.Module("principalresolver",
	fx.Provide(
		NewPrincipalResolver,
		fx.Annotate(
			ProvideAsUCANOption,
			fx.ResultTags(`group:"ucan_options"`),
		),
		fx.Annotate(
			ProvideAsUCANRetrievalOption,
			fx.ResultTags(`group:"ucan_retrieval_options"`),
		),
	),
)

// NewPrincipalResolver creates a principal resolver from configuration
func NewPrincipalResolver(cfg app.AppConfig) (validator.PrincipalResolver, error) {
	services := make([]did.DID, 0, 2)
	if idxSvc := cfg.UCANService.Services.Indexer.Connection; idxSvc != nil {
		if strings.HasPrefix(idxSvc.ID().DID().String(), "did:web:") {
			services = append(services, idxSvc.ID().DID())
		}
	}
	if uplSvc := cfg.UCANService.Services.Upload.Connection; uplSvc != nil {
		if strings.HasPrefix(uplSvc.ID().DID().String(), "did:web:") {
			services = append(services, uplSvc.ID().DID())
		}
	}
	hr, err := principalresolver.NewHTTPResolver(services)
	if err != nil {
		return nil, fmt.Errorf("creating http principal resolver: %w", err)
	}
	cr, err := principalresolver.NewCachedResolver(hr, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("creating cached principal resolver: %w", err)
	}
	return cr, nil
}

// ProvideAsUCANOption provides the principal resolver as a UCAN server option
func ProvideAsUCANOption(resolver validator.PrincipalResolver) ucanserver.Option {
	return ucanserver.WithPrincipalResolver(resolver.ResolveDIDKey)
}

// ProvideAsUCANRetrievalOption provides the principal resolver as a UCAN
// retrieval server option/
func ProvideAsUCANRetrievalOption(resolver validator.PrincipalResolver) ucanretrievalserver.Option {
	return ucanretrievalserver.WithPrincipalResolver(resolver.ResolveDIDKey)
}
