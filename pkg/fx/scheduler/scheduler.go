package scheduler

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
)

var Module = fx.Module("scheduler",
	fx.Provide(
		ProvideChainScheduler,
		ProvideEngine,
	),
	fx.Invoke(func(e *scheduler.TaskEngine) {
		e.SessionID()
	}),
	MessageModule,
	TasksModule,
)

type EngineParams struct {
	fx.In

	DB    *gorm.DB                  `name:"engine_db"`
	Tasks []scheduler.TaskInterface `group:"scheduler_tasks"`
}

func ProvideEngine(lc fx.Lifecycle, params EngineParams) (*scheduler.TaskEngine, error) {
	engine, err := scheduler.NewEngine(params.DB, params.Tasks)
	if err != nil {
		return nil, fmt.Errorf("creating scheduler engine: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return engine.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return engine.Stop(ctx)
		},
	})

	return engine, nil
}

func ProvideChainScheduler(lc fx.Lifecycle, client service.ChainClient) (*chainsched.Scheduler, error) {
	cs, err := chainsched.New(client)
	if err != nil {
		return nil, fmt.Errorf("creating chain scheduler: %w", err)
	}

	csCtx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go cs.Run(csCtx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancel()
			return nil
		},
	})

	return cs, nil
}
