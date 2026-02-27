package principalresolver

import (
	"fmt"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/storacha/go-ucanto/did"
	ucanserver "github.com/storacha/go-ucanto/server"
	ucanretrievalserver "github.com/storacha/go-ucanto/server/retrieval"
	"github.com/storacha/go-ucanto/validator"
	"go.uber.org/fx"

	"github.com/storacha/piri/pkg/config/app"
	"github.com/storacha/piri/pkg/principalresolver"
)

var log = logging.Logger("fx/principalresolver")

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
	logging.SetAllLoggers(logging.LevelInfo)
	services := make([]did.DID, 0, 2)

	log.Info("Creating principal resolver from config")

	if idxSvc := cfg.UCANService.Services.Indexer.Connection; idxSvc != nil {
		idxDID := idxSvc.ID().DID().String()
		log.Infof("Indexer service DID: %s", idxDID)
		if strings.HasPrefix(idxDID, "did:web:") {
			services = append(services, idxSvc.ID().DID())
			log.Infof("Added indexer DID to resolver: %s", idxDID)
		} else {
			log.Infof("Indexer DID is not did:web, skipping: %s", idxDID)
		}
	} else {
		log.Warn("Indexer service connection is nil - no indexer DID configured")
	}

	if uplSvc := cfg.UCANService.Services.Upload.Connection; uplSvc != nil {
		uplDID := uplSvc.ID().DID().String()
		log.Infof("Upload service DID: %s", uplDID)
		if strings.HasPrefix(uplDID, "did:web:") {
			services = append(services, uplSvc.ID().DID())
			log.Infof("Added upload DID to resolver: %s", uplDID)
		} else {
			log.Infof("Upload DID is not did:web, skipping: %s", uplDID)
		}
	} else {
		log.Warn("Upload service connection is nil - no upload DID configured")
	}

	log.Infof("Principal resolver will resolve %d DIDs", len(services))

	// TODO: make insecure resolution configurable based on network (e.g., local-net)
	// For now, always use insecure (HTTP) resolution to support local development
	hr, err := principalresolver.NewHTTPResolver(services, principalresolver.InsecureResolution())
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
