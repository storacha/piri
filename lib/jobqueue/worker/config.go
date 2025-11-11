package worker

import (
	"time"

	"github.com/storacha/piri/lib/jobqueue/types"
)

// Config holds all parameters needed to initialize a Worker.
type Config struct {
	Log           types.StandardLogger
	JobCountLimit int
	PollInterval  time.Duration
	Extend        time.Duration
}

// Option modifies a Config before creating the Worker.
type Option func(*Config)

func WithLog(l types.StandardLogger) Option {
	return func(cfg *Config) {
		cfg.Log = l
	}
}

func WithLimit(limit int) Option {
	return func(cfg *Config) {
		cfg.JobCountLimit = limit
	}
}

func WithPollInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		cfg.PollInterval = interval
	}
}

func WithExtend(d time.Duration) Option {
	return func(cfg *Config) {
		cfg.Extend = d
	}
}
