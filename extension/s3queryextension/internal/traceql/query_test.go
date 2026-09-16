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

func TestParseTraceQLDurationWithDot(t *testing.T) {
	// .duration > 100ms should parse to the same MinDurationNano as duration > 100ms
	q1, err := ParseTraceQL(`{ .duration > 100ms }`)
	if err != nil {
		t.Fatal(err)
	}
	q2, err := ParseTraceQL(`{ duration > 100ms }`)
	if err != nil {
		t.Fatal(err)
	}
	if q1.MinDurationNano != q2.MinDurationNano || q1.MinDurationNano != 100_000_000 {
		t.Fatalf("duration mismatch: .duration=%d, duration=%d", q1.MinDurationNano, q2.MinDurationNano)
	}
}

func TestParseTraceQLUnsupportedOperators(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"duration <", `{ duration < 100ms }`},
		{"duration <=", `{ duration <= 100ms }`},
		{"duration =", `{ duration = 100ms }`},
		{"duration !=", `{ duration != 100ms }`},
		{"status !=", `{ status != error }`},
		{"status >", `{ status > error }`},
		{"status <", `{ status < error }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTraceQL(tt.query)
			if err == nil {
				t.Fatalf("expected error for query %q, got nil", tt.query)
			}
		})
	}
}
