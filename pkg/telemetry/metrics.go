package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/storacha/piri/pkg/build"
)

// RecordServerInfo records server metadata as an info metric
// this metric is best effort, if it fails, a warning is log and the process continues
func RecordServerInfo(ctx context.Context, serverType string, extraAttrs ...attribute.KeyValue) {
	info, err := Global().NewInfo(InfoConfig{
		Name:        "piri_server_info",
		Description: "Build and runtime information about the Piri server",
	})
	if err != nil {
		log.Warnw("failed to initialize piri server info metric", "error", err, "type", serverType)
	}

	// Base attributes that all servers share
	attrs := []attribute.KeyValue{
		StringAttr("version", build.Version),
		StringAttr("commit", build.Commit),
		StringAttr("built_by", build.BuiltBy),
		StringAttr("build_date", build.Date),
		Int64Attr("start_time_unix", time.Now().Unix()),
		StringAttr("server_type", serverType),
	}

	// Add any server-specific attributes
	attrs = append(attrs, extraAttrs...)

	info.Record(ctx, attrs...)
}
