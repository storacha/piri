package scheduler

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/chainsched"
	"github.com/storacha/piri/pkg/pdp/ethereum"
	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service"
	"github.com/storacha/piri/pkg/pdp/smartcontracts"
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
		//   and Sender can't exist until SendTaskETH exists. So we make them both together
		ProvideSenderETHPair,
		fx.Annotate(
			ProvideSenderFromPair,
			fx.As(new(ethereum.Sender)),
		),
		fx.Annotate(
			ProvideSendTaskFromPair,
			fx.ResultTags(`group:"scheduler_tasks"`),
			fx.As(new(scheduler.TaskInterface)),
		),
	),
	// NB: these methods are invoked as they do not provide any types in their return or nothing depends on their return
	fx.Invoke(
		StartWatcherMessageEth,
		StartWatcherCreate,
		StartWatcherRootAdd,
		StartWatcherProviderRegister,
	),
)

type SenderETHParams struct {
	fx.In
	DB     *gorm.DB `name:"engine_db"`
	Client service.EthClient
	Wallet wallet.Wallet
}

// SenderETHPair holds both the sender and task to ensure they're created together
type SenderETHPair struct {
	Sender   *tasks.SenderETH
	SendTask *tasks.SendTaskETH
}

func ProvideSenderETHPair(params SenderETHParams) *SenderETHPair {
	sender, sendTask := tasks.NewSenderETH(params.Client, params.Wallet, params.DB)
	return &SenderETHPair{
		Sender:   sender,
		SendTask: sendTask,
	}
}

func ProvideSenderFromPair(pair *SenderETHPair) *tasks.SenderETH {
	return pair.Sender
}

func ProvideSendTaskFromPair(pair *SenderETHPair) *tasks.SendTaskETH {
	return pair.SendTask
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
	DB          *gorm.DB `name:"engine_db"`
	Verifier    smartcontracts.Verifier
	Scheduler   *chainsched.Scheduler
	ServiceView smartcontracts.Service
}

func StartWatcherCreate(params WatcherCreateParams) error {
	return tasks.NewWatcherCreate(
		params.DB,
		params.Verifier,
		params.Scheduler,
		params.ServiceView,
	)
}

type WatcherRootAddParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Verifier  smartcontracts.Verifier
	Scheduler *chainsched.Scheduler
}

func StartWatcherRootAdd(params WatcherRootAddParams) error {
	return tasks.NewWatcherRootAdd(
		params.DB,
		params.Scheduler,
		params.Verifier,
	)
}

type WatcherProviderRegisterParams struct {
	fx.In
	DB        *gorm.DB `name:"engine_db"`
	Scheduler *chainsched.Scheduler
}

func StartWatcherProviderRegister(params WatcherProviderRegisterParams) error {
	return tasks.NewWatcherProviderRegister(
		params.DB,
		params.Scheduler,
	)
}
