package egresstracking

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/storacha/piri/pkg/pdp/aggregator/jobqueue"
)

type EgressTrackingQueue interface {
	Register(fn func(ctx context.Context, batchCID cid.Cid) error) error
	Enqueue(ctx context.Context, batchCID cid.Cid) error
}

const egressTrackTaskName = "egress-track-task"

var _ EgressTrackingQueue = (*jobQueueAdapter)(nil)

type jobQueueAdapter struct {
	queue *jobqueue.JobQueue[cid.Cid]
}

func NewEgressTrackingQueue(queue *jobqueue.JobQueue[cid.Cid]) EgressTrackingQueue {
	return &jobQueueAdapter{queue: queue}
}

func (a *jobQueueAdapter) Register(fn func(ctx context.Context, batchCID cid.Cid) error) error {
	return a.queue.Register(egressTrackTaskName, fn)
}

func (a *jobQueueAdapter) Enqueue(ctx context.Context, batchCID cid.Cid) error {
	return a.queue.Enqueue(ctx, egressTrackTaskName, batchCID)
}
