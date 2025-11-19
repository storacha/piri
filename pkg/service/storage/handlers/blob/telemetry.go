package blob

import (
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("github.com/storacha/piri/pkg/service/storage/handlers/blob")
)
