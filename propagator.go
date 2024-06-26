package datadog

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDHeaderKey  = "x-datadog-trace-id"
	parentIDHeaderKey = "x-datadog-parent-id"
	priorityHeaderKey = "x-datadog-sampling-priority"
	// TODO: other headers: https://github.com/DataDog/dd-trace-go/blob/4f0b6ac22e14082ee1443d502a35a99cd9459ee0/ddtrace/tracer/textmap.go#L73-L96

	// These are typical sampling values for Datadog, but Datadog libraries actually support any integer values
	// values >=1 mean the trace is sampled, values <= 0 mean the trace is not sampled
	notSampled = "0"
	isSampled  = "1"
)

var (
	empty = trace.SpanContext{}

	errMalformedTraceID              = errors.New("cannot parse Datadog trace ID as 64bit unsigned int from header")
	errMalformedSpanID               = errors.New("cannot parse Datadog span ID as 64bit unsigned int from header")
	errInvalidTraceIDHeader          = errors.New("invalid Datadog trace ID header found")
	errInvalidSpanIDHeader           = errors.New("invalid Datadog span ID header found")
	errInvalidSamplingPriorityHeader = errors.New("invalid Datadog sampling priority header found")
)

// Propagator serializes Span Context to/from Datadog headers.
//
// Example Datadog format:
// X-Datadog-Trace-Id: 16701352862047361693
// X-Datadog-Parent-Id: 2939011537882399028
type Propagator struct{}

// Asserts that the propagator implements the otel.TextMapPropagator interface at compile time.
var _ propagation.TextMapPropagator = &Propagator{}

// Inject injects a context to the carrier following Datadog format.
func (dd Propagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.TraceID().IsValid() || !sc.SpanID().IsValid() {
		return
	}

	traceID := sc.TraceID().String()
	parentID := sc.SpanID().String()
	if traceID == "" || parentID == "" {
		// invalid data
		return
	}

	samplingFlag := notSampled
	if sc.IsSampled() {
		samplingFlag = isSampled
	}

	carrier.Set(traceIDHeaderKey, convertOTtoDD(traceID))
	carrier.Set(parentIDHeaderKey, convertOTtoDD(parentID))
	carrier.Set(priorityHeaderKey, samplingFlag)
}

// Extract gets a context from the carrier if it contains Datadog headers.
func (dd Propagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	var (
		traceID = carrier.Get(traceIDHeaderKey)
		spanID  = carrier.Get(parentIDHeaderKey)
		sampled = carrier.Get(priorityHeaderKey)
	)
	sc, err := extract(traceID, spanID, sampled)
	if err != nil || !sc.IsValid() {
		return ctx
	}

	return trace.ContextWithRemoteSpanContext(ctx, sc)
}

func extract(traceID, spanID, sampled string) (trace.SpanContext, error) {
	var (
		scc = trace.SpanContextConfig{}
		err error
		ok  bool
	)

	if traceID, ok = convertDDtoOT(traceID); !ok {
		return empty, errMalformedTraceID
	}

	if traceID != "" {
		id := traceID
		if len(traceID) < 32 {
			// Pad 64-bit trace IDs
			id = fmt.Sprintf("%032s", traceID)
		}
		if scc.TraceID, err = trace.TraceIDFromHex(id); err != nil {
			return empty, errInvalidTraceIDHeader
		}
	}

	if spanID, ok = convertDDtoOT(spanID); !ok {
		return empty, errMalformedSpanID
	}

	if spanID != "" {
		id := spanID
		if len(spanID) < 16 {
			// Pad 64-bit span IDs
			id = fmt.Sprintf("%016s", spanID)
		}
		if scc.SpanID, err = trace.SpanIDFromHex(id); err != nil {
			return empty, errInvalidSpanIDHeader
		}
	}

	sampledInt, err := strconv.Atoi(sampled)
	if err != nil {
		return empty, errInvalidSamplingPriorityHeader
	}
	if sampledInt >= 1 {
		scc.TraceFlags = trace.FlagsSampled
	}

	return trace.NewSpanContext(scc), nil
}

// Fields returns list of fields set with Inject.
func (dd Propagator) Fields() []string {
	return []string{
		traceIDHeaderKey,
		parentIDHeaderKey,
		priorityHeaderKey,
	}
}

// convert OpenTelemetry trace and span IDs to Datadog IDs
// Datadog IDs are limited to 64-bits:
// https://docs.datadoghq.com/tracing/guide/send_traces_to_agent_by_api/#send-traces
// Code originally from:
// https://docs.datadoghq.com/tracing/connect_logs_and_traces/opentelemetry/
// X-B3-Traceid: [b810dba29803ee61e7c71ff0c2c95a9d] to 16701352862047361693
// Ot-Tracer-Traceid: [e7c71ff0c2c95a9d] to 16701352862047361693
func convertOTtoDD(id string) string {
	if len(id) < 16 {
		return ""
	}
	// For trace IDs longer than 64-bits / 16 characters, only the last 64-bits / 16 characters are used
	// For example, B3 supports both 128 or 64-bit IDs using 32 or 16 lower-hex characters respectively
	// https://github.com/openzipkin/b3-propagation#traceid-1
	if len(id) > 16 {
		id = id[16:]
	}
	intValue, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(intValue, 10)
}

// convert Datadog trace and span IDs to OpenTelemetry IDs
// X-Datadog-Trace-Id: [16701352862047361693] to e7c71ff0c2c95a9d
// X-Datadog-Parent-Id: [2939011537882399028] to 28c9776c12414134
func convertDDtoOT(id string) (string, bool) {
	u, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return "", false
	}

	return strconv.FormatUint(u, 16), true
}
