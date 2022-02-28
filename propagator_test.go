package datadog

import (
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/stretchr/testify/assert"
)

var (
	// 000000000000000079d48a391a778fa6
	traceID   = trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0x79, 0xd4, 0x8a, 0x39, 0x1a, 0x77, 0x8f, 0xa6}
	ddTraceID = "8778793551513751462"

	// 53995c3f42cd8ad8
	spanID     = trace.SpanID{0x53, 0x99, 0x5c, 0x3f, 0x42, 0xcd, 0x8a, 0xd8}
	ddParentID = "6023947403358210776"

	// 000000000000000000000000075bcd15
	traceIDSmall   = trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x07, 0x5b, 0xcd, 0x15}
	ddTraceIDSmall = "123456789"

	// 000000003ade68b1
	spanIDSmall     = trace.SpanID{0, 0, 0, 0, 0x3a, 0xde, 0x68, 0xb1}
	ddParentIDSmall = "987654321"
)

func TestExtractMultiple(t *testing.T) {
	tests := []struct {
		traceID  string
		spanID   string
		sampled  string
		expected trace.SpanContextConfig
		err      error
	}{
		{
			ddTraceID, ddParentID, notSampled,
			trace.SpanContextConfig{
				TraceID: traceID,
				SpanID:  spanID,
			},
			nil,
		},
		{
			ddTraceID, ddParentID, isSampled,
			trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
			},
			nil,
		},
		{
			ddTraceIDSmall, ddParentIDSmall, "",
			trace.SpanContextConfig{
				TraceID: traceIDSmall,
				SpanID:  spanIDSmall,
			},
			nil,
		},
		{
			"", ddParentID, "",
			trace.SpanContextConfig{},
			errMalformedTraceID,
		},
		{
			ddTraceID, "", "",
			trace.SpanContextConfig{},
			errMalformedSpanID,
		},
		{
			"0000000000000000000", ddParentID, "",
			trace.SpanContextConfig{},
			errInvalidTraceIDHeader,
		},
		{
			ddTraceID, "0000000000000000000", "",
			trace.SpanContextConfig{},
			errInvalidSpanIDHeader,
		},
	}

	for _, test := range tests {
		actual, err := extract(
			test.traceID,
			test.spanID,
			test.sampled,
		)
		info := []interface{}{
			"trace ID: %q, span ID: %q, sampled: %q",
			test.traceID,
			test.spanID,
			test.sampled,
		}
		if !assert.Equal(t, test.err, err, info...) {
			continue
		}
		assert.Equal(t, trace.NewSpanContext(test.expected), actual, info...)
	}
}
