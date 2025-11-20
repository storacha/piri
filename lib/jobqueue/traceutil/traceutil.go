package traceutil

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel/trace"
)

type linkContextKey struct{}

// ContextWithLink stores a link on the context without setting a parent.
func ContextWithLink(ctx context.Context, sc trace.SpanContext) context.Context {
	if !sc.IsValid() {
		return ctx
	}

	link := trace.Link{SpanContext: makeRemote(sc)}

	return context.WithValue(ctx, linkContextKey{}, link)
}

// LinkFromContext retrieves a span link added by the worker when a trace context was present at enqueue time.
func LinkFromContext(ctx context.Context) (trace.Link, bool) {
	link, ok := ctx.Value(linkContextKey{}).(trace.Link)
	return link, ok
}

// StartSpan creates a new span that is linked (not parented) to the stored trace context if present.
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if link, ok := LinkFromContext(ctx); ok {
		opts = append(opts, trace.WithLinks(link))
	}

	// Drop any parent span to avoid creating parent-child relationships across queue boundaries.
	ctx = trace.ContextWithSpanContext(ctx, trace.SpanContext{})
	return tracer.Start(ctx, name, opts...)
}

// SpanContextPayload is a lightweight representation of a span context for persistence.
type SpanContextPayload struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceFlags uint8  `json:"trace_flags,omitempty"`
	TraceState string `json:"trace_state,omitempty"`
}

func makeRemote(sc trace.SpanContext) trace.SpanContext {
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    sc.TraceID(),
		SpanID:     sc.SpanID(),
		TraceFlags: sc.TraceFlags(),
		TraceState: sc.TraceState(),
		Remote:     true,
	})
}

// PayloadFromContext builds a payload from the span context on the provided context.
func PayloadFromContext(ctx context.Context) *SpanContextPayload {
	return PayloadFromSpanContext(trace.SpanFromContext(ctx).SpanContext())
}

// PayloadFromSpanContext builds a payload from a span context.
func PayloadFromSpanContext(sc trace.SpanContext) *SpanContextPayload {
	if !sc.IsValid() {
		return nil
	}
	payload := &SpanContextPayload{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}
	if sc.TraceFlags() != 0 {
		payload.TraceFlags = uint8(sc.TraceFlags())
	}
	if ts := sc.TraceState().String(); ts != "" {
		payload.TraceState = ts
	}
	return payload
}

// SpanContextFromPayload reconstructs a span context from a payload.
func SpanContextFromPayload(payload *SpanContextPayload) (trace.SpanContext, bool) {
	if payload == nil {
		return trace.SpanContext{}, false
	}

	traceID, err := trace.TraceIDFromHex(payload.TraceID)
	if err != nil {
		return trace.SpanContext{}, false
	}

	spanID, err := trace.SpanIDFromHex(payload.SpanID)
	if err != nil {
		return trace.SpanContext{}, false
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.TraceFlags(payload.TraceFlags),
	})

	if payload.TraceState != "" {
		if ts, err := trace.ParseTraceState(payload.TraceState); err == nil {
			sc = sc.WithTraceState(ts)
		}
	}

	return sc, sc.IsValid()
}

func MarshalPayload(payload *SpanContextPayload) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}
	return json.Marshal(payload)
}

func UnmarshalPayload(b []byte) (*SpanContextPayload, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var payload SpanContextPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
