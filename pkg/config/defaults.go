package config

import (
	"runtime"
	"time"

	"github.com/spf13/viper"
)

// DefaultMinimumEgressBatchSize is the minimum allowed egress tracker batch size (10 MiB).
const DefaultMinimumEgressBatchSize int64 = 10 * 1024 * 1024

// Key is a configuration key path used with Viper.
type Key string

// PDP Aggregation - CommP
const (
	CommPJobQueueWorkers    Key = "pdp.aggregation.commp.job_queue.workers"
	CommPJobQueueRetries    Key = "pdp.aggregation.commp.job_queue.retries"
	CommPJobQueueRetryDelay Key = "pdp.aggregation.commp.job_queue.retry_delay"
)

// PDP Aggregation - Aggregator
const (
	AggregatorJobQueueWorkers    Key = "pdp.aggregation.aggregator.job_queue.workers"
	AggregatorJobQueueRetries    Key = "pdp.aggregation.aggregator.job_queue.retries"
	AggregatorJobQueueRetryDelay Key = "pdp.aggregation.aggregator.job_queue.retry_delay"
)

// PDP Aggregation - Manager (these are dynamic - can change at runtime)
const (
	ManagerPollInterval       Key = "pdp.aggregation.manager.poll_interval"
	ManagerBatchSize          Key = "pdp.aggregation.manager.batch_size"
	ManagerJobQueueWorkers    Key = "pdp.aggregation.manager.job_queue.workers"
	ManagerJobQueueRetries    Key = "pdp.aggregation.manager.job_queue.retries"
	ManagerJobQueueRetryDelay Key = "pdp.aggregation.manager.job_queue.retry_delay"
)

// PDP Gas Fee Limits (dynamic - can change at runtime)
const (
	GasMaxFeeProve         Key = "pdp.gas.max_fee.prove"
	GasMaxFeeProvingPeriod Key = "pdp.gas.max_fee.proving_period"
	GasMaxFeeProvingInit   Key = "pdp.gas.max_fee.proving_init"
	GasMaxFeeAddRoots      Key = "pdp.gas.max_fee.add_roots"
	GasMaxFeeDefault       Key = "pdp.gas.max_fee.default"
	GasRetryWait           Key = "pdp.gas.retry_wait"
)

var defaultValues = map[Key]any{
	CommPJobQueueWorkers:    runtime.NumCPU(),
	CommPJobQueueRetries:    50,
	CommPJobQueueRetryDelay: 10 * time.Second,

	AggregatorJobQueueWorkers:    runtime.NumCPU(),
	AggregatorJobQueueRetries:    50,
	AggregatorJobQueueRetryDelay: 10 * time.Second,

	ManagerPollInterval:       30 * time.Second,
	ManagerBatchSize:          10,
	ManagerJobQueueWorkers:    3,
	ManagerJobQueueRetries:    50,
	ManagerJobQueueRetryDelay: time.Minute,
}

// SetDefaults sets all viper defaults for configuration.
// Called before viper.Unmarshal() to ensure defaults are available.
func SetDefaults() {
	for k, v := range defaultValues {
		viper.SetDefault(string(k), v)
	}
}
