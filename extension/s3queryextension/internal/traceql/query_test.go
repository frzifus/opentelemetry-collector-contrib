package traceql

import "testing"

func TestParseTraceQLBasic(t *testing.T) {
	q, err := ParseTraceQL(`{ .service.name = "checkout" && duration > 100ms && .http.method = "GET" }`)
	if err != nil {
		t.Fatal(err)
	}
	if q.ServiceName != "checkout" {
		t.Fatalf("service = %q", q.ServiceName)
	}
	if q.MinDurationNano != 100_000_000 {
		t.Fatalf("minDuration = %d", q.MinDurationNano)
	}
	if len(q.Attrs) != 1 || q.Attrs[0].Field != "http.method" || q.Attrs[0].Str != "GET" {
		t.Fatalf("attrs = %+v", q.Attrs)
	}
}

func TestParseTraceQLStatus(t *testing.T) {
	q, err := ParseTraceQL(`{ status = error }`)
	if err != nil {
		t.Fatal(err)
	}
	if q.StatusCode == nil || *q.StatusCode != 2 {
		t.Fatalf("status = %v", q.StatusCode)
	}
}
