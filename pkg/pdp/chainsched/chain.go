package chainsched

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/clock"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/store"
	"github.com/filecoin-project/lotus/chain/types"
)

var log = logging.Logger("scheduler/chain")

// Notification timeout for chain updates, if we don't get a notification within this time frame
// then something must be wrong so we'll attempt to restart
const notificationTimeout = 60 * time.Second

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

func (s *Scheduler) Run(ctx context.Context) {
	s.lk.Lock()
	s.started = true
	s.lk.Unlock()

	var (
		notificationCh       <-chan []*api.HeadChange
		err                  error
		gotFirstNotification bool
	)

	ticker := s.clock.Ticker(notificationTimeout)
	defer ticker.Stop()
	lastNotif := s.clock.Now()

	// not fine to panic after this point
	for ctx.Err() == nil {
		if notificationCh == nil {
			notificationCh, err = s.api.ChainNotify(ctx)
			if err != nil {
				log.Errorw("ChainNotify", "error", err)
				s.clock.Sleep(10 * time.Second) // Retry after 10 second wait
				continue
			}
			gotFirstNotification = false
			log.Info("restarting Scheduler with new notification channel")
			lastNotif = s.clock.Now()
		}

		select {
		case changes, ok := <-notificationCh:
			if !ok {
				log.Warn("chain notification channel closed")
				notificationCh = nil
				continue
			}

			notifSummaries := make([]string, len(changes))
			for i, chg := range changes {
				var height int64 = -1
				if chg.Val != nil {
					height = int64(chg.Val.Height())
				}
				notifSummaries[i] = fmt.Sprintf("[%d:%v:h=%d]", i, chg.Type, height)
			}
			log.Debugf("received notification: %d changes %v", len(changes), notifSummaries)

			lastNotif = s.clock.Now()

			if !gotFirstNotification {
				if len(changes) != 1 {
					log.Errorf("expected first chain notification to have a single change")
					notificationCh = nil
					s.clock.Sleep(10 * time.Second) // Retry after 10 second wait
					continue
				}
				chg := changes[0]
				if chg.Type != store.HCCurrent {
					log.Errorf(`expected first chain notification to tell "current" TipSet`)
					notificationCh = nil
					s.clock.Sleep(10 * time.Second) // Retry after 10 second wait
					continue
				}

				s.update(ctx, nil, chg.Val)

				gotFirstNotification = true
				continue
			}

			var lowest, highest *types.TipSet = nil, nil

			for _, change := range changes {
				if change.Val == nil {
					log.Errorf("change.Val was nil")
				}
				switch change.Type {
				case store.HCRevert:
					lowest = change.Val
				case store.HCApply:
					highest = change.Val
				}
			}

			s.update(ctx, lowest, highest)

		case <-ticker.C:
			since := s.clock.Since(lastNotif)
			log.Debugf("Scheduler ticker: %s since last notification", since)
			if since > notificationTimeout {
				log.Warnf("no notifications received in %s, resubscribing to ChainNotify", notificationTimeout)
				notificationCh = nil
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) update(ctx context.Context, revert, apply *types.TipSet) {
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
}
