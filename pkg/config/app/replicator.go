package app

import (
	"runtime"
	"time"
)

type ReplicatorConfig struct {
	// MaxRetries configures the maximum retries allowed by the replication queue
	MaxRetries uint
	// MaxWorkers configures the maximum workers ran by the replication queue
	MaxWorkers uint
	// MaxTimeout configures timeout for jobs before they can be re-evaluated
	MaxTimeout time.Duration
}

func DefaultReplicatorConfig() ReplicatorConfig {
	return ReplicatorConfig{
		MaxWorkers: uint(runtime.NumCPU()),
		MaxRetries: 10,
		MaxTimeout: 5 * time.Second,
	}
}
