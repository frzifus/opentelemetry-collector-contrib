// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// promotedKeys is the set of span-attribute keys that map to dedicated columns
// (service.name is sourced from the resource, handled separately).
var promotedKeys = map[string]struct{}{
	ColSpanName: {}, ColSpanKind: {}, ColDurationNano: {}, ColStatusCode: {},
}

func TracesToRows(td ptrace.Traces) []SpanRow {
	var rows []SpanRow
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		svc := ""
		if v, ok := rs.Resource().Attributes().Get(ColServiceName); ok {
			svc = v.AsString()
		}
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				rows = append(rows, spanToRow(spans.At(k), svc, rs.Resource().Attributes()))
			}
		}
	}
	return rows
}

func spanToRow(sp ptrace.Span, svc string, resAttrs pcommon.Map) SpanRow {
	r := SpanRow{
		TraceID:       idBytes(sp.TraceID()),
		SpanID:        spanIDBytes(sp.SpanID()),
		ParentSpanID:  spanIDBytes(sp.ParentSpanID()),
		ServiceName:   svc,
		SpanName:      sp.Name(),
		SpanKind:      int32(sp.Kind()),
		StartUnixNano: int64(sp.StartTimestamp()),
		DurationNano:  int64(sp.EndTimestamp()) - int64(sp.StartTimestamp()),
		StatusCode:    int32(sp.Status().Code()),
		AttrsStr:      map[string]string{},
		AttrsInt:      map[string]int64{},
		AttrsDouble:   map[string]float64{},
		AttrsBool:     map[string]bool{},
		AttrsBytes:    map[string][]byte{},
		AttrsComplex:  map[string][]byte{},
	}
	sp.Attributes().Range(func(key string, v pcommon.Value) bool {
		if _, isPromoted := promotedKeys[key]; isPromoted {
			return true
		}
		putTyped(&r, key, v)
		return true
	})
	return r
}

func putTyped(r *SpanRow, key string, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		r.AttrsStr[key] = v.Str()
	case pcommon.ValueTypeInt:
		r.AttrsInt[key] = v.Int()
	case pcommon.ValueTypeDouble:
		r.AttrsDouble[key] = v.Double()
	case pcommon.ValueTypeBool:
		r.AttrsBool[key] = v.Bool()
	case pcommon.ValueTypeBytes:
		r.AttrsBytes[key] = v.Bytes().AsRaw()
	default: // Slice, Map, Empty
		r.AttrsComplex[key] = marshalComplex(v)
	}
}

func idBytes(id pcommon.TraceID) []byte  { b := id; return b[:] }
func spanIDBytes(id pcommon.SpanID) []byte { b := id; return b[:] }

func RowsToTraces(rows []SpanRow) ptrace.Traces {
	td := ptrace.NewTraces()
	// Group by service into one ResourceSpans each for simplicity (POC).
	byService := map[string]ptrace.SpanSlice{}
	for _, r := range rows {
		spans, ok := byService[r.ServiceName]
		if !ok {
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr(ColServiceName, r.ServiceName)
			spans = rs.ScopeSpans().AppendEmpty().Spans()
			byService[r.ServiceName] = spans
		}
		rowToSpan(r, spans.AppendEmpty())
	}
	return td
}

func rowToSpan(r SpanRow, sp ptrace.Span) {
	var tid pcommon.TraceID
	copy(tid[:], r.TraceID)
	var sid, pid pcommon.SpanID
	copy(sid[:], r.SpanID)
	copy(pid[:], r.ParentSpanID)
	sp.SetTraceID(tid)
	sp.SetSpanID(sid)
	sp.SetParentSpanID(pid)
	sp.SetName(r.SpanName)
	sp.SetKind(ptrace.SpanKind(r.SpanKind))
	sp.SetStartTimestamp(pcommon.Timestamp(r.StartUnixNano))
	sp.SetEndTimestamp(pcommon.Timestamp(r.StartUnixNano + r.DurationNano))
	sp.Status().SetCode(ptrace.StatusCode(r.StatusCode))
	for k, v := range r.AttrsStr {
		sp.Attributes().PutStr(k, v)
	}
	for k, v := range r.AttrsInt {
		sp.Attributes().PutInt(k, v)
	}
	for k, v := range r.AttrsDouble {
		sp.Attributes().PutDouble(k, v)
	}
	for k, v := range r.AttrsBool {
		sp.Attributes().PutBool(k, v)
	}
	for k, v := range r.AttrsBytes {
		sp.Attributes().PutEmptyBytes(k).FromRaw(v)
	}
	for k, v := range r.AttrsComplex {
		unmarshalComplexInto(sp.Attributes(), k, v)
	}
}
