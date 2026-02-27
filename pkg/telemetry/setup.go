package telemetry

import (
	"context"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconvhttp "go.opentelemetry.io/otel/semconv/v1.37.0/httpconv"

	"github.com/storacha/piri/lib/telemetry"
	"github.com/storacha/piri/lib/telemetry/metrics"
	"github.com/storacha/piri/lib/telemetry/traces"
	"github.com/storacha/piri/pkg/build"
)

const (
	defaultEndpoint        = "telemetry.storacha.network:443"
	defaultPublishInterval = 30 * time.Second
)

func Setup(ctx context.Context, network string, id string) (*telemetry.Telemetry, error) {
	if network == "" {
		log.Warn("network not configured; telemetry will use 'custom' as deployment environment")
		network = "custom"
	}
	return telemetry.New(
		ctx,
		network,
		"piri",
		build.Version,
		id,
		metrics.Config{
			Collectors: []metrics.CollectorConfig{
				{
					Endpoint:        defaultEndpoint,
					PublishInterval: defaultPublishInterval,
				},
			},
			Options: []sdkmetric.Option{
				sdkmetric.WithView(
					// custom views for http metics with more buckets for histograms
					DefaultHTTPServicerRequestDurationView,
					DefaultHTTPServerRequestBodySizeView,
					DefaultHTTPServerResponseBodySizeView,
				),
			},
		},
		traces.Config{
			Collectors: []traces.CollectorConfig{
				{
					Endpoint:        defaultEndpoint,
					PublishInterval: defaultPublishInterval,
				},
			},
			Options: []sdktrace.TracerProviderOption{
				// Only sample when there is a parent trace; never start local roots.
				sdktrace.WithSampler(
					sdktrace.ParentBased(sdktrace.NeverSample()),
				),
			},
		},
	)
}

var HTTPServerDurationBounds = []float64{
	(5 * time.Millisecond).Seconds(),
	(10 * time.Millisecond).Seconds(),
	(100 * time.Millisecond).Seconds(),
	(time.Second).Seconds(),
	(3 * time.Second).Seconds(),
	(5 * time.Second).Seconds(),
	(10 * time.Second).Seconds(),
	(30 * time.Second).Seconds(),
	(time.Minute).Seconds(),
	(2 * time.Minute).Seconds(),
	(3 * time.Minute).Seconds(),
	(5 * time.Minute).Seconds(),
	(6 * time.Minute).Seconds(),
	(7 * time.Minute).Seconds(),
	(8 * time.Minute).Seconds(),
	(9 * time.Minute).Seconds(),
	(10 * time.Minute).Seconds(),
}

const (
	KiB float64 = 1024
	MiB         = KiB * 1024
	GiB         = MiB * 1024
)

var SizeBoundaries = []float64{
	// Explicit histogram buckets for request/response body sizes (bytes), up to 1 GiB.
	KiB,
	2 * KiB,
	4 * KiB,
	8 * KiB,
	16 * KiB,
	32 * KiB,
	64 * KiB,
	128 * KiB,
	256 * KiB,
	512 * KiB,
	MiB,
	2 * MiB,
	4 * MiB,
	8 * MiB,
	16 * MiB,
	32 * MiB,
	64 * MiB,
	128 * MiB,
	256 * MiB,
	512 * MiB,
	GiB,
}

var (
	DefaultHTTPServicerRequestDurationView = sdkmetric.NewView(
		sdkmetric.Instrument{
			Name:        semconvhttp.ServerRequestDuration{}.Name(),
			Description: semconvhttp.ServerRequestDuration{}.Description(),
			Kind:        sdkmetric.InstrumentKindHistogram,
			Unit:        semconvhttp.ServerRequestDuration{}.Unit(),
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: HTTPServerDurationBounds,
			},
		},
	)
	DefaultHTTPServerRequestBodySizeView = sdkmetric.NewView(
		sdkmetric.Instrument{
			Name:        semconvhttp.ServerRequestBodySize{}.Name(),
			Description: semconvhttp.ServerRequestBodySize{}.Description(),
			Kind:        sdkmetric.InstrumentKindHistogram,
			Unit:        semconvhttp.ServerRequestBodySize{}.Unit(),
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: SizeBoundaries,
			},
		},
	)
	DefaultHTTPServerResponseBodySizeView = sdkmetric.NewView(
		sdkmetric.Instrument{
			Name:        semconvhttp.ServerResponseBodySize{}.Name(),
			Description: semconvhttp.ServerResponseBodySize{}.Description(),
			Kind:        sdkmetric.InstrumentKindHistogram,
			Unit:        semconvhttp.ServerResponseBodySize{}.Unit(),
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: SizeBoundaries,
			},
		},
	)
)
