package principalresolver

import (
	"fmt"
	"time"

	"github.com/storacha/go-ucanto/did"
	ucanserver "github.com/storacha/go-ucanto/server"
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
	),
)

// NewPrincipalResolver creates a principal resolver from configuration
func NewPrincipalResolver(cfg app.AppConfig) (validator.PrincipalResolver, error) {
	services := make([]did.DID, 0, 2)
	if idxSvc := cfg.External.IndexingService.Connection; idxSvc != nil {
		services = append(services, idxSvc.ID().DID())
	}
	if uplSvc := cfg.External.UploadService.Connection; uplSvc != nil {
		services = append(services, uplSvc.ID().DID())
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
