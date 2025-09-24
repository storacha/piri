package scheduler

import (
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
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
			ProvidePDPProveTask,
			fx.As(new(scheduler.TaskInterface)),
			fx.ResultTags(`group:"scheduler_tasks"`),
		),
	),
)

type InitProvingPeriodTaskParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  smartcontracts.PDP
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
	Contract  smartcontracts.PDP
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

type PDPProveTaskParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  smartcontracts.PDP
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
