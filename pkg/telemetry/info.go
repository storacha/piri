package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/storacha/piri/lib/telemetry"
	"github.com/storacha/piri/pkg/build"
)

func RecordServerInfo(meter metric.Meter, ctx context.Context, serverType string, extraAttrs ...attribute.KeyValue) error {
	allAttrs := append(extraAttrs,
		attribute.String("version", build.Version),
		attribute.String("commit", build.Commit),
		attribute.String("built_by", build.BuiltBy),
		attribute.String("build_date", build.Date),
		attribute.Int64("start_time_unix", time.Now().Unix()),
		attribute.String("server_type", serverType),
	)
	info, err := telemetry.NewInfo(
		meter,
		"piri_server_info",
		"Build and runtime information about the Piri server",
		allAttrs...,
	)
	if err != nil {
		return err
	}
	info.Record(ctx)
	return nil
}
