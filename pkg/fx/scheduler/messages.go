package scheduler

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/service/contract"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/wallet"
)

var MessageModule = fx.Module("scheduler-messages",
	fx.Provide(
		// This setup is required to prevent a circular dependency
		// - SenderETH (implements ethereum.Sender) depends on SendTaskETH
		// - SendTaskETH is registered as a scheduler task
		// - Other tasks (InitProvingPeriodTask, NextProvingPeriodTask, ProveTask) depend on ethereum.Sender (SenderETH)
		// - Fx needs all tasks created before building the engine, but can't create tasks that depend on Sender until Sender exists
		//   and Sender can't exist until SendTaskETH exists. So we make them both together and:
		//     - label the task as a scheduler_tasks group, making it available to the scheduler.
		//     - annotate the SenderETH as an ethereum.Sender, the interface it implements.
		fx.Annotate(
			ProvideSenderETH,
			fx.As(new(ethereum.Sender)),                  // First result as Sender interface
			fx.ResultTags(``, `group:"scheduler_tasks"`), // Second result to task group
		),
	),
	// NB: these methods are invoked as they do not provide any types in their return or nothing depends on their return
	fx.Invoke(
		StartWatcherMessageEth,
		StartWatcherCreate,
		StartWatcherRootAdd,
	),
)

type SenderETHParams struct {
	fx.In
	DB     *gorm.DB `name:"engine_db"`
	Client service.EthClient
	Wallet wallet.Wallet
}

func ProvideSenderETH(params SenderETHParams) (*tasks.SenderETH, *tasks.SendTaskETH) {
	return tasks.NewSenderETH(params.Client, params.Wallet, params.DB)
}

type WatcherMessageEthParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Scheduler *chainsched.Scheduler
}

func StartWatcherMessageEth(
	lc fx.Lifecycle,
	params WatcherMessageEthParams,
) (*tasks.MessageWatcherEth, error) {
	ew, err := tasks.NewMessageWatcherEth(params.DB, params.Scheduler, params.Client)
	if err != nil {
		return nil, fmt.Errorf("creating message watcher: %w", err)
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ew.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return ew.Stop(ctx)
		},
	})
	return ew, nil
}

type WatcherCreateParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Client    service.EthClient
	Contract  contract.PDP
	Scheduler *chainsched.Scheduler
}

func StartWatcherCreate(params WatcherCreateParams) error {
	return tasks.NewWatcherCreate(
		params.DB,
		params.Client,
		params.Contract,
		params.Scheduler,
	)
}

type WatcherRootAddParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Contract  contract.PDP
	Scheduler *chainsched.Scheduler
}

func StartWatcherRootAdd(params WatcherRootAddParams) error {
	return tasks.NewWatcherRootAdd(
		params.DB,
		params.Scheduler,
		params.Contract,
	)
}
