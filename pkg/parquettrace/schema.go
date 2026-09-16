// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"

// SchemaVersion identifies the on-disk span schema. Recorded in every block's meta.
const SchemaVersion = "1"

// Column names for promoted (dedicated) columns.
const (
	ColTraceID       = "trace_id"
	ColSpanID        = "span_id"
	ColParentSpanID  = "parent_span_id"
	ColStartUnixNano = "start_unix_nano"
	ColServiceName   = "service.name"
	ColSpanName      = "span.name"
	ColSpanKind      = "span.kind"
	ColDurationNano  = "duration_nano"
	ColStatusCode    = "status.code"
)

// AlwaysPromoted columns are structural and always written as dedicated columns.
var AlwaysPromoted = []string{ColTraceID, ColSpanID, ColParentSpanID, ColStartUnixNano}

// DefaultPromoted are the configurable-but-default dedicated attribute columns.
var DefaultPromoted = []string{ColServiceName, ColSpanName, ColSpanKind, ColDurationNano, ColStatusCode}

// DefaultBloom columns get a native Parquet bloom filter by default.
var DefaultBloom = []string{ColTraceID}

// SpanRow is one span, one row. Generic (non-promoted) attributes live in the typed maps.
type SpanRow struct {
	TraceID      []byte `parquet:"trace_id"`
	SpanID       []byte `parquet:"span_id"`
	ParentSpanID []byte `parquet:"parent_span_id"`
	ServiceName  string `parquet:"service.name,dict"`
	SpanName     string `parquet:"span.name,dict"`
	SpanKind     int32  `parquet:"span.kind"`
	StartUnixNano int64 `parquet:"start_unix_nano"`
	DurationNano int64  `parquet:"duration_nano"`
	StatusCode   int32  `parquet:"status.code"`

	AttrsStr     map[string]string  `parquet:"attrs_str"`
	AttrsInt     map[string]int64   `parquet:"attrs_int"`
	AttrsDouble  map[string]float64 `parquet:"attrs_double"`
	AttrsBool    map[string]bool    `parquet:"attrs_bool"`
	AttrsBytes   map[string][]byte  `parquet:"attrs_bytes"`
	AttrsComplex map[string][]byte  `parquet:"attrs_complex"`
}
