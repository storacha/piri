package commp

import (
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("github.com/storacha/piri/pkg/pdp/aggregation/commp")
)
