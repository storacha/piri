package chainsched

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-state-types/builtin"
	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/clock"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
)

var log = logging.Logger("scheduler/chain")

// Notification timeout for chain updates, if we don't get a notification within this time frame
// then something must be wrong so we'll attempt to restart. 3 epochs to account for null rounds.
const notificationTimeout = 3 * (builtin.EpochDurationSeconds * time.Second)

type NodeAPI interface {
	ChainHead(context.Context) (*types.TipSet, error)
	ChainNotify(context.Context) (<-chan []*api.HeadChange, error)
}

type Scheduler struct {
	api NodeAPI

	callbacks []UpdateFunc
	lk        sync.RWMutex
	started   bool
	clock     clock.Clock
}

type Option func(*Scheduler)

func WithClock(clock clock.Clock) Option {
	return func(s *Scheduler) {
		s.clock = clock
	}
}

func New(api NodeAPI, opts ...Option) *Scheduler {
	s := &Scheduler{
		api:   api,
		clock: clock.New(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type UpdateFunc func(ctx context.Context, revert, apply *types.TipSet) error

func (s *Scheduler) AddHandler(ch UpdateFunc) error {
	s.lk.Lock()
	defer s.lk.Unlock()
	if s.started {
		return xerrors.Errorf("cannot add handler after start")
	}
	s.callbacks = append(s.callbacks, ch)
	return nil
}

// chainSubscription manages a single chain notification subscription
type chainSubscription struct {
	ch     <-chan []*api.HeadChange
	cancel context.CancelFunc
}

// close cancels the subscription context
func (cs *chainSubscription) close() {
	if cs.cancel != nil {
		cs.cancel()
	}
}

// Run subscribes to chain notifications and processes them until the context is cancelled
func (s *Scheduler) Run(ctx context.Context) {
	s.markStarted()

	state := &schedulerState{
		lastNotification: s.clock.Now(),
		isInitialized:    false,
	}

	timeoutChecker := s.clock.Ticker(notificationTimeout)
	defer timeoutChecker.Stop()

	var subscription *chainSubscription
	defer func() {
		if subscription != nil {
			subscription.close()
		}
	}()

	for ctx.Err() == nil {
		// Establish subscription if needed
		if subscription == nil || subscription.ch == nil {
			subscription = s.establishSubscription(ctx, subscription)
			if subscription == nil {
				continue
			}
			state.reset(s.clock.Now())
		}

		// Process events
		select {
		case changes, ok := <-subscription.ch:
			if !ok {
				log.Warn("chain notification channel closed")
				subscription.ch = nil
				continue
			}
			s.handleNotification(ctx, changes, state)

		case <-timeoutChecker.C:
			if s.isSubscriptionTimedOut(state.lastNotification) {
				log.Warnf("no notifications received in %s, resubscribing to ChainNotify", notificationTimeout)
				subscription.ch = nil
			}

		case <-ctx.Done():
			return
		}
	}
}

// schedulerState tracks the state of the scheduler
type schedulerState struct {
	lastNotification time.Time
	isInitialized    bool
}

func (ss *schedulerState) reset(now time.Time) {
	ss.lastNotification = now
	ss.isInitialized = false
}

// markStarted marks the scheduler as started
func (s *Scheduler) markStarted() {
	s.lk.Lock()
	s.started = true
	s.lk.Unlock()
}

// establishSubscription creates a new chain notification subscription
func (s *Scheduler) establishSubscription(ctx context.Context, oldSub *chainSubscription) *chainSubscription {
	// Clean up old subscription
	if oldSub != nil {
		oldSub.close()
	}

	// Create new subscription
	subCtx, cancel := context.WithCancel(ctx)
	notificationCh, err := s.api.ChainNotify(subCtx)
	if err != nil {
		log.Errorw("ChainNotify", "error", err)
		cancel()
		s.clock.Sleep(10 * time.Second)
		return nil
	}

	log.Info("established new chain notification subscription")
	return &chainSubscription{
		ch:     notificationCh,
		cancel: cancel,
	}
}

// handleNotification processes a chain notification
func (s *Scheduler) handleNotification(ctx context.Context, changes []*api.HeadChange, state *schedulerState) {
	s.logNotification(changes)
	state.lastNotification = s.clock.Now()

	if !state.isInitialized {
		if err := s.handleInitialNotification(ctx, changes); err != nil {
			log.Errorf("failed to handle initial notification: %v", err)
			s.clock.Sleep(10 * time.Second)
			return
		}
		state.isInitialized = true
		return
	}

	s.processChainChanges(ctx, changes)
}

// handleInitialNotification processes the first notification after subscription
func (s *Scheduler) handleInitialNotification(ctx context.Context, changes []*api.HeadChange) error {
	if len(changes) != 1 {
		return xerrors.Errorf("expected first chain notification to have a single change, got %d", len(changes))
	}

	change := changes[0]
	if change.Type != store.HCCurrent {
		return xerrors.Errorf("expected first chain notification to be HCCurrent, got %v", change.Type)
	}

	s.update(ctx, nil, change.Val)
	return nil
}

// processChainChanges processes regular chain change notifications
func (s *Scheduler) processChainChanges(ctx context.Context, changes []*api.HeadChange) {
	var revertTo, applyFrom *types.TipSet

	for _, change := range changes {
		if change.Val == nil {
			log.Errorf("received change with nil tipset")
			continue
		}

		switch change.Type {
		case store.HCRevert:
			revertTo = change.Val
		case store.HCApply:
			applyFrom = change.Val
		}
	}

	s.update(ctx, revertTo, applyFrom)
}

// isSubscriptionTimedOut checks if the subscription has timed out
func (s *Scheduler) isSubscriptionTimedOut(lastNotif time.Time) bool {
	since := s.clock.Since(lastNotif)
	log.Debugf("Scheduler ticker: %s since last notification", since)
	return since > notificationTimeout
}

// logNotification logs debug information about received notifications
func (s *Scheduler) logNotification(changes []*api.HeadChange) {
	notifSummaries := make([]string, len(changes))
	for i, chg := range changes {
		height := int64(-1)
		if chg.Val != nil {
			height = int64(chg.Val.Height())
		}
		notifSummaries[i] = fmt.Sprintf("[%d:%v:h=%d]", i, chg.Type, height)
	}
	log.Debugf("received notification: %d changes %v", len(changes), notifSummaries)
}

func (s *Scheduler) update(ctx context.Context, revert, apply *types.TipSet) {
	start := s.clock.Now()
	log.Debugw("start chain scheduler update", "apply", apply.Height())
	if apply == nil {
		log.Error("no new tipset in Scheduler.update")
		return
	}

	s.lk.RLock()
	callbacksCopy := make([]UpdateFunc, len(s.callbacks))
	copy(callbacksCopy, s.callbacks)
	s.lk.RUnlock()

	for _, ch := range callbacksCopy {
		if err := ch(ctx, revert, apply); err != nil {
			log.Errorf("handling head updates in Scheduler: %+v", err)
		}
	}
	log.Debugw("end chain scheduler update", "duration", time.Since(start), "apply", apply.Height())
}
