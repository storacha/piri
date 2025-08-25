package service

import (
	"fmt"
	"math"
	"time"
)

// CleanupConfig holds configuration for cleanup operations
type CleanupConfig struct {
	// Stash cleanup settings
	StashCleanupEnabled  bool          `json:"stash_cleanup_enabled" yaml:"stash_cleanup_enabled"`
	StashCleanupInterval time.Duration `json:"stash_cleanup_interval" yaml:"stash_cleanup_interval"`
	StashRetentionPeriod time.Duration `json:"stash_retention_period" yaml:"stash_retention_period"`

	// Blob cleanup settings
	BlobCleanupEnabled  bool          `json:"blob_cleanup_enabled" yaml:"blob_cleanup_enabled"`
	BlobCleanupInterval time.Duration `json:"blob_cleanup_interval" yaml:"blob_cleanup_interval"`

	// Startup cleanup settings
	StartupCleanupEnabled bool `json:"startup_cleanup_enabled" yaml:"startup_cleanup_enabled"`

	// General cleanup settings
	MaxRetries             int     `json:"max_retries" yaml:"max_retries"`
	RetryBackoffMultiplier float64 `json:"retry_backoff_multiplier" yaml:"retry_backoff_multiplier"`
	BatchSize              int     `json:"batch_size" yaml:"batch_size"`

	// Metrics settings
	MetricsEnabled  bool          `json:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsInterval time.Duration `json:"metrics_interval" yaml:"metrics_interval"`
}

// DefaultCleanupConfig returns the default cleanup configuration
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		StashCleanupEnabled:  true,
		StashCleanupInterval: 30 * time.Second,
		StashRetentionPeriod: 24 * time.Hour,

		BlobCleanupEnabled:  true,
		BlobCleanupInterval: 5 * time.Minute,

		StartupCleanupEnabled: true,

		MaxRetries:             3,
		RetryBackoffMultiplier: 2.0,
		BatchSize:              100,

		MetricsEnabled:  true,
		MetricsInterval: 1 * time.Minute,
	}
}

// Validate validates the cleanup configuration
func (c *CleanupConfig) Validate() error {
	if c.StashCleanupInterval <= 0 {
		return fmt.Errorf("stash_cleanup_interval must be positive")
	}

	if c.BlobCleanupInterval <= 0 {
		return fmt.Errorf("blob_cleanup_interval must be positive")
	}

	if c.StashRetentionPeriod <= 0 {
		return fmt.Errorf("stash_retention_period must be positive")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}

	if c.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf("retry_backoff_multiplier must be positive")
	}

	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}

	if c.MetricsInterval <= 0 {
		return fmt.Errorf("metrics_interval must be positive")
	}

	return nil
}

// GetRetryWaitDuration calculates the retry wait duration based on retry count
func (c *CleanupConfig) GetRetryWaitDuration(retryCount int) time.Duration {
	if retryCount <= 0 {
		return 0
	}

	baseDuration := time.Duration(retryCount) * 5 * time.Minute
	multiplier := math.Pow(c.RetryBackoffMultiplier, float64(retryCount-1))

	return time.Duration(float64(baseDuration) * multiplier)
}
