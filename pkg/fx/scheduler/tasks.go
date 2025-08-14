package scheduler

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/store/blobstore"
)

var TasksModule = fx.Module("scheduler-tasks",
	fx.Provide(
		fx.Annotate(
			ProvideInitProvingPeriodTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			ProvideNextProvingPeriodTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			ProvidePDPNotifyTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			ProvidePDPProveTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
		fx.Annotate(
			ProvideStorePieceTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
	),
)

type InitProvingPeriodTaskParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  contract.PDP
	Chain     service.ChainClient
	Scheduler *chainsched.Scheduler
	Sender    ethereum.Sender
}

func ProvideInitProvingPeriodTask(params InitProvingPeriodTaskParams) (*tasks.InitProvingPeriodTask, error) {
	return tasks.NewInitProvingPeriodTask(
		params.DB,
		params.Client,
		params.Contract,
		params.Chain,
		params.Scheduler,
		params.Sender,
	)
}

type NextProvingPeriodTaskParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  contract.PDP
	Chain     service.ChainClient
	Scheduler *chainsched.Scheduler
	Sender    ethereum.Sender
}

func ProvideNextProvingPeriodTask(params NextProvingPeriodTaskParams) (*tasks.NextProvingPeriodTask, error) {
	return tasks.NewNextProvingPeriodTask(
		params.DB,
		params.Client,
		params.Contract,
		params.Chain,
		params.Scheduler,
		params.Sender,
	)
}

type PDPNotifyTaskParams struct {
	fx.In
	DB *gorm.DB `name:"engine_db"`
}

func ProvidePDPNotifyTask(params PDPNotifyTaskParams) *tasks.PDPNotifyTask {
	return tasks.NewPDPNotifyTask(params.DB)
}

type PDPProveTaskParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  contract.PDP
	Chain     service.ChainClient
	Scheduler *chainsched.Scheduler
	Sender    ethereum.Sender
	Store     blobstore.PDPStore
}

func ProvidePDPProveTask(params PDPProveTaskParams) (*tasks.ProveTask, error) {
	return tasks.NewProveTask(
		params.Scheduler,
		params.DB,
		params.Client,
		params.Contract,
		params.Chain,
		params.Sender,
		params.Store,
	)
}

type StorePieceTaskParams struct {
	fx.In
	DB    *gorm.DB `name:"engine_db"`
	Store blobstore.PDPStore
}

func ProvideStorePieceTask(lc fx.Lifecycle, params StorePieceTaskParams) *tasks.ParkPieceTask {
	t := tasks.NewStorePieceTask(params.DB, params.Store)
	tctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			t.Start(tctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
	return t
}
