package ucan

import (
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("github.com/storacha/piri/pkg/service/retrieval/ucan")
)
