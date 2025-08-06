package scheduler

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/tasks"
)

var Module = fx.Module("scheduler",
	fx.Provide(
		ProvideEngine,
		ProvideChainScheduler,
	),
	TasksModule,
	MessageModule,
)

var TasksModule = fx.Module("scheduler-tasks",
	fx.Provide(
		fx.Annotate(
			tasks.NewSenderTaskETH,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			tasks.NewInitProvingPeriodTask,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			tasks.NewNextProvingPeriodTask,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			tasks.NewPDPNotifyTask,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			tasks.NewProveTask,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			tasks.NewStorePieceTask,
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
	),
)

var MessageModule = fx.Module("scheduler-messages",
	fx.Provide(
		tasks.NewSenderETH,
		ProvideWatcherMessageEth,
	),
	fx.Invoke(
		tasks.NewWatcherCreate,
		tasks.NewWatcherRootAdd,
	),
)

func ProvideWatcherMessageEth(
	lc fx.Lifecycle,
	db *gorm.DB,
	cs *chainsched.Scheduler,
	client service.EthClient,
) (*tasks.MessageWatcherEth, error) {
	ew, err := tasks.NewMessageWatcherEth(db, cs, client)
	if err != nil {
		return nil, fmt.Errorf("creating message watcher: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return ew.Stop(ctx)
		},
	})
	return ew, nil
}

type EngineParams struct {
	fx.In

	DB    *gorm.DB
	Tasks []scheduler.TaskInterface `group:"scheduler_tasks"`
}

func ProvideEngine(lc fx.Lifecycle, params EngineParams) (*scheduler.TaskEngine, error) {
	engine, err := scheduler.NewEngine(params.DB, params.Tasks)
	if err != nil {
		return nil, fmt.Errorf("creating scheduler engine: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return models.AutoMigrateDB(ctx, params.DB)
		},
		OnStop: func(ctx context.Context) error {
			engine.GracefullyTerminate()
			return nil
		},
	})

	return engine, nil
}

func ProvideChainScheduler(lc fx.Lifecycle, client service.ChainClient) (*chainsched.Scheduler, error) {
	cs := chainsched.New(client)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			cs.Run(context.TODO())
			return nil
		},
	})

	return cs, nil
}
