package telemetry

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// StartHostMetrics exports basic host metrics (CPU, memory, and data-dir disk usage)
// via the global meter. Metrics are intentionally scoped to avoid PII; only the
// data-dir path is attached to disk metrics so we can distinguish the storage
// volume being monitored.
func StartHostMetrics(ctx context.Context, dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("dataDir is required to start host metrics")
	}

	meter := Global().Meter()

	cpuUtilization, err := meter.Float64ObservableGauge(
		"system_cpu_utilization",
		metric.WithDescription("System-wide CPU utilization as a fraction (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("create cpu utilization gauge: %w", err)
	}

	memUsed, err := meter.Int64ObservableGauge(
		"system_memory_used_bytes",
		metric.WithDescription("System memory used in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("create memory used gauge: %w", err)
	}

	memTotal, err := meter.Int64ObservableGauge(
		"system_memory_total_bytes",
		metric.WithDescription("System memory total in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("create memory total gauge: %w", err)
	}

	dataDirUsed, err := meter.Int64ObservableGauge(
		"piri_datadir_used_bytes",
		metric.WithDescription("Bytes used on the filesystem backing the Piri data-dir"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("create data-dir used gauge: %w", err)
	}

	dataDirFree, err := meter.Int64ObservableGauge(
		"piri_datadir_free_bytes",
		metric.WithDescription("Free bytes on the filesystem backing the Piri data-dir"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("create data-dir free gauge: %w", err)
	}

	dataDirTotal, err := meter.Int64ObservableGauge(
		"piri_datadir_total_bytes",
		metric.WithDescription("Total bytes on the filesystem backing the Piri data-dir"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return fmt.Errorf("create data-dir total gauge: %w", err)
	}

	dataDirAttr := attribute.String("path", dataDir)

	reg, err := meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			if percentages, err := cpu.Percent(0, false); err == nil && len(percentages) > 0 {
				// cpu.Percent returns 0-100; convert to 0-1 for utilization.
				o.ObserveFloat64(cpuUtilization, percentages[0]/100.0)
			}

			if vm, err := mem.VirtualMemory(); err == nil {
				o.ObserveInt64(memUsed, int64(vm.Used))
				o.ObserveInt64(memTotal, int64(vm.Total))
			}

			if usage, err := disk.Usage(dataDir); err == nil {
				attrOpt := metric.WithAttributes(dataDirAttr)
				o.ObserveInt64(dataDirUsed, int64(usage.Used), attrOpt)
				o.ObserveInt64(dataDirFree, int64(usage.Free), attrOpt)
				o.ObserveInt64(dataDirTotal, int64(usage.Total), attrOpt)
			}

			return nil
		},
		cpuUtilization,
		memUsed,
		memTotal,
		dataDirUsed,
		dataDirFree,
		dataDirTotal,
	)
	if err != nil {
		return fmt.Errorf("register host metrics callback: %w", err)
	}

	go func() {
		<-ctx.Done()
		if err := reg.Unregister(); err != nil {
			log.Debugw("failed to unregister host metrics callback", "error", err)
		}
	}()

	return nil
}
