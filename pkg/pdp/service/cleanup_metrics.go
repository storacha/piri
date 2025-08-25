package service

import (
	"sync/atomic"
	"time"
)

// CleanupMetrics tracks cleanup operation statistics
type CleanupMetrics struct {
	StashCleanupSuccess   int64
	StashCleanupFailure   int64
	BlobCleanupSuccess    int64
	BlobCleanupFailure    int64
	StartupCleanupSuccess int64
	StartupCleanupFailure int64
	SpaceReclaimedBytes   int64
	LastCleanupTime       int64 // Unix timestamp
}

// CleanupMetricsService provides metrics tracking for cleanup operations
type CleanupMetricsService struct {
	metrics CleanupMetrics
}

// NewCleanupMetricsService creates a new cleanup metrics service
func NewCleanupMetricsService() *CleanupMetricsService {
	return &CleanupMetricsService{
		metrics: CleanupMetrics{
			LastCleanupTime: time.Now().Unix(),
		},
	}
}

// RecordStashCleanup records a stash cleanup operation
func (c *CleanupMetricsService) RecordStashCleanup(success bool, bytesReclaimed int64) {
	if success {
		atomic.AddInt64(&c.metrics.StashCleanupSuccess, 1)
		atomic.AddInt64(&c.metrics.SpaceReclaimedBytes, bytesReclaimed)
	} else {
		atomic.AddInt64(&c.metrics.StashCleanupFailure, 1)
	}
	atomic.StoreInt64((*int64)(&c.metrics.LastCleanupTime), time.Now().Unix())
}

// RecordBlobCleanup records a blob cleanup operation
func (c *CleanupMetricsService) RecordBlobCleanup(success bool, bytesReclaimed int64) {
	if success {
		atomic.AddInt64(&c.metrics.BlobCleanupSuccess, 1)
		atomic.AddInt64(&c.metrics.SpaceReclaimedBytes, bytesReclaimed)
	} else {
		atomic.AddInt64(&c.metrics.BlobCleanupFailure, 1)
	}
	atomic.StoreInt64((*int64)(&c.metrics.LastCleanupTime), time.Now().Unix())
}

// RecordStartupCleanup records a startup cleanup operation
func (c *CleanupMetricsService) RecordStartupCleanup(success bool, filesRemoved int64) {
	if success {
		atomic.AddInt64(&c.metrics.StartupCleanupSuccess, 1)
	} else {
		atomic.AddInt64(&c.metrics.StartupCleanupFailure, 1)
	}
	atomic.StoreInt64((*int64)(&c.metrics.LastCleanupTime), time.Now().Unix())
}

// GetMetrics returns a copy of the current metrics
func (c *CleanupMetricsService) GetMetrics() CleanupMetrics {
	return CleanupMetrics{
		StashCleanupSuccess:   atomic.LoadInt64(&c.metrics.StashCleanupSuccess),
		StashCleanupFailure:   atomic.LoadInt64(&c.metrics.StashCleanupFailure),
		BlobCleanupSuccess:    atomic.LoadInt64(&c.metrics.BlobCleanupSuccess),
		BlobCleanupFailure:    atomic.LoadInt64(&c.metrics.BlobCleanupFailure),
		StartupCleanupSuccess: atomic.LoadInt64(&c.metrics.StartupCleanupSuccess),
		StartupCleanupFailure: atomic.LoadInt64(&c.metrics.StartupCleanupFailure),
		SpaceReclaimedBytes:   atomic.LoadInt64(&c.metrics.SpaceReclaimedBytes),
		LastCleanupTime:       atomic.LoadInt64(&c.metrics.LastCleanupTime),
	}
}

// ResetMetrics resets all metrics to zero
func (c *CleanupMetricsService) ResetMetrics() {
	atomic.StoreInt64(&c.metrics.StashCleanupSuccess, 0)
	atomic.StoreInt64(&c.metrics.StashCleanupFailure, 0)
	atomic.StoreInt64(&c.metrics.BlobCleanupSuccess, 0)
	atomic.StoreInt64(&c.metrics.BlobCleanupFailure, 0)
	atomic.StoreInt64(&c.metrics.StartupCleanupSuccess, 0)
	atomic.StoreInt64(&c.metrics.StartupCleanupFailure, 0)
	atomic.StoreInt64(&c.metrics.SpaceReclaimedBytes, 0)
	atomic.StoreInt64((*int64)(&c.metrics.LastCleanupTime), time.Now().Unix())
}
