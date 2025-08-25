package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/pdp/tasks"
	"github.com/storacha/piri/pkg/store/blobstore"
	"github.com/storacha/piri/pkg/store/stashstore"
)

// CleanupService orchestrates all cleanup operations
type CleanupService struct {
	db             *gorm.DB
	blobstore      blobstore.Blobstore
	stashstore     stashstore.Stash
	config         *CleanupConfig
	metrics        *CleanupMetricsService
	startupCleanup *StartupCleanupService
	cleanupTask    *tasks.CleanupTask
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(
	db *gorm.DB,
	blobstore blobstore.Blobstore,
	stashstore stashstore.Stash,
	config *CleanupConfig,
) *CleanupService {
	metrics := NewCleanupMetricsService()
	startupCleanup := NewStartupCleanupService(db, stashstore)
	cleanupTask := tasks.NewCleanupTask(db, blobstore, stashstore)

	return &CleanupService{
		db:             db,
		blobstore:      blobstore,
		stashstore:     stashstore,
		config:         config,
		metrics:        metrics,
		startupCleanup: startupCleanup,
		cleanupTask:    cleanupTask,
	}
}

// Start initializes and starts the cleanup service
func (c *CleanupService) Start(ctx context.Context) error {
	// Validate configuration
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("invalid cleanup configuration: %w", err)
	}

	// Run startup cleanup if enabled
	if c.config.StartupCleanupEnabled {
		if err := c.runStartupCleanup(ctx); err != nil {
			log.Errorw("startup cleanup failed", "error", err)
			// Don't fail the service start, just log the error
		}
	}

	// Start the cleanup task
	c.cleanupTask.Start(ctx)

	log.Info("cleanup service started successfully")
	return nil
}

// Stop stops the cleanup service
func (c *CleanupService) Stop(ctx context.Context) error {
	log.Info("stopping cleanup service")
	// The cleanup task will stop when the context is cancelled
	return nil
}

// runStartupCleanup runs the startup cleanup process
func (c *CleanupService) runStartupCleanup(ctx context.Context) error {
	log.Info("running startup cleanup")

	start := time.Now()
	err := c.startupCleanup.CleanupOrphanedStashes(ctx)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordStartupCleanup(false, 0)
		return fmt.Errorf("startup cleanup failed: %w", err)
	}

	c.metrics.RecordStartupCleanup(true, 0)
	log.Infow("startup cleanup completed", "duration", duration)
	return nil
}

// RegisterWithScheduler registers the cleanup task with a scheduler
func (c *CleanupService) RegisterWithScheduler(addTaskFunc scheduler.AddTaskFunc) {
	// Set up the task adder
	c.cleanupTask.Adder(addTaskFunc)
	log.Info("cleanup task registered with scheduler")
}

// GetMetrics returns the current cleanup metrics
func (c *CleanupService) GetMetrics() CleanupMetrics {
	return c.metrics.GetMetrics()
}

// ResetMetrics resets all cleanup metrics
func (c *CleanupService) ResetMetrics() {
	c.metrics.ResetMetrics()
}

// GetConfig returns the current cleanup configuration
func (c *CleanupService) GetConfig() *CleanupConfig {
	return c.config
}

// UpdateConfig updates the cleanup configuration
func (c *CleanupService) UpdateConfig(config *CleanupConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	c.config = config
	log.Info("cleanup configuration updated")
	return nil
}

// ManualCleanup triggers a manual cleanup operation
func (c *CleanupService) ManualCleanup(ctx context.Context) error {
	log.Info("manual cleanup triggered")

	// Run startup cleanup
	if err := c.runStartupCleanup(ctx); err != nil {
		return fmt.Errorf("manual startup cleanup failed: %w", err)
	}

	log.Info("manual cleanup completed")
	return nil
}

// GetCleanupStatus returns the current status of cleanup operations
func (c *CleanupService) GetCleanupStatus(ctx context.Context) (*CleanupStatus, error) {
	// Get metrics
	metrics := c.metrics.GetMetrics()

	// Get pending cleanup counts
	pendingStashCount, err := c.getPendingStashCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting pending stash count: %w", err)
	}

	pendingBlobCount, err := c.getPendingBlobCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting pending blob count: %w", err)
	}

	return &CleanupStatus{
		Metrics:           metrics,
		PendingStashCount: pendingStashCount,
		PendingBlobCount:  pendingBlobCount,
		Config:            c.config,
		LastUpdated:       time.Now(),
	}, nil
}

// getPendingStashCount returns the number of stash files pending cleanup
func (c *CleanupService) getPendingStashCount(ctx context.Context) (int64, error) {
	var count int64
	err := c.db.WithContext(ctx).
		Model(&models.ParkedPiece{}).
		Where("complete = TRUE AND cleanup_task_id IS NULL").
		Count(&count).Error
	return count, err
}

// getPendingBlobCount returns the number of blobs pending cleanup
func (c *CleanupService) getPendingBlobCount(ctx context.Context) (int64, error) {
	var count int64
	err := c.db.WithContext(ctx).
		Model(&models.PDPPieceRef{}).
		Where("proofset_refcount = 0").
		Count(&count).Error
	return count, err
}

// CleanupStatus represents the current status of cleanup operations
type CleanupStatus struct {
	Metrics           CleanupMetrics `json:"metrics"`
	PendingStashCount int64          `json:"pending_stash_count"`
	PendingBlobCount  int64          `json:"pending_blob_count"`
	Config            *CleanupConfig `json:"config"`
	LastUpdated       time.Time      `json:"last_updated"`
}
