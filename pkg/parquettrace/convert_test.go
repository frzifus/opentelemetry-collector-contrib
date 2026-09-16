// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestRoundTripSingleSpan(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	ss := rs.ScopeSpans().AppendEmpty()
	sp := ss.Spans().AppendEmpty()
	sp.SetTraceID([16]byte{1})
	sp.SetSpanID([8]byte{2})
	sp.SetName("GET /cart")
	sp.SetKind(ptrace.SpanKindServer)
	sp.SetStartTimestamp(100)
	sp.SetEndTimestamp(400)
	sp.Attributes().PutInt("http.status_code", 200)
	sp.Attributes().PutStr("http.method", "GET")

	rows := TracesToRows(td)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.ServiceName != "checkout" || r.SpanName != "GET /cart" {
		t.Fatalf("promoted fields wrong: %+v", r)
	}
	if r.DurationNano != 300 {
		t.Fatalf("duration = %d, want 300", r.DurationNano)
	}
	if r.AttrsInt["http.status_code"] != 200 {
		t.Fatalf("attrs_int missing http.status_code: %+v", r.AttrsInt)
	}
	if r.AttrsStr["http.method"] != "GET" {
		t.Fatalf("attrs_str missing http.method: %+v", r.AttrsStr)
	}

	back := RowsToTraces(rows)
	got := back.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	if got.Name() != "GET /cart" {
		t.Fatalf("roundtrip name = %q", got.Name())
	}
	if v, ok := got.Attributes().Get("http.status_code"); !ok || v.Int() != 200 {
		t.Fatalf("roundtrip attr lost")
	}
}
