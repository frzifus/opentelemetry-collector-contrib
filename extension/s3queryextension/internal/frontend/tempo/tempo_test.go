// tempo_test.go
package tempo

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func TestSearchHandlerReturnsTempoJSON(t *testing.T) {
	rows := []parquettrace.SpanRow{
		{TraceID: []byte{0xaa}, SpanID: []byte{1}, ServiceName: "checkout", SpanName: "root", StartUnixNano: 10, DurationNano: 200_000_000},
	}
	data, meta, err := parquettrace.WriteBlock(rows, parquettrace.DefaultBloom)
	if err != nil { t.Fatal(err) }
	meta.Path = "traces/blk.parquet"
	m := &store.MemStore{Objects: map[string][]byte{"traces/blk.parquet": data, "traces/blk.meta.json": mustMeta(t, meta)}}
	s := &store.Store{OS: m, Prefix: "traces/"}

	mux := http.NewServeMux()
	New(s).Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + `/api/search?q=` + `%7B%20.service.name%20%3D%20%22checkout%22%20%7D`)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Fatalf("status %d", resp.StatusCode) }
	var out struct{ Traces []struct{ TraceID string `json:"traceID"` } `json:"traces"` }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { t.Fatal(err) }
	if len(out.Traces) != 1 || out.Traces[0].TraceID != hex.EncodeToString([]byte{0xaa}) {
		t.Fatalf("search json = %+v", out)
	}
}

func TestSearchHandlerLimitZero(t *testing.T) {
	// Create a block with multiple traces
	rows := []parquettrace.SpanRow{
		{TraceID: []byte{0xaa}, SpanID: []byte{1}, ServiceName: "svc", SpanName: "root1", StartUnixNano: 10, DurationNano: 100},
		{TraceID: []byte{0xbb}, SpanID: []byte{2}, ServiceName: "svc", SpanName: "root2", StartUnixNano: 20, DurationNano: 100},
		{TraceID: []byte{0xcc}, SpanID: []byte{3}, ServiceName: "svc", SpanName: "root3", StartUnixNano: 30, DurationNano: 100},
	}
	data, meta, err := parquettrace.WriteBlock(rows, parquettrace.DefaultBloom)
	if err != nil { t.Fatal(err) }
	meta.Path = "traces/blk.parquet"
	m := &store.MemStore{Objects: map[string][]byte{"traces/blk.parquet": data, "traces/blk.meta.json": mustMeta(t, meta)}}
	s := &store.Store{OS: m, Prefix: "traces/"}

	mux := http.NewServeMux()
	New(s).Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Test ?limit=0 should use default (20) not unbounded
	resp, err := http.Get(srv.URL + `/api/search?limit=0`)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Fatalf("status %d", resp.StatusCode) }
	var out struct{ Traces []struct{ TraceID string } }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { t.Fatal(err) }
	if len(out.Traces) > maxSearchLimit {
		t.Fatalf("expected at most %d traces with limit=0, got %d", maxSearchLimit, len(out.Traces))
	}
}

func TestSearchHandlerLimitFive(t *testing.T) {
	// Create a block with 10 traces
	rows := make([]parquettrace.SpanRow, 10)
	for i := 0; i < 10; i++ {
		rows[i] = parquettrace.SpanRow{
			TraceID: []byte{byte(i)}, SpanID: []byte{byte(i)}, ServiceName: "svc", SpanName: "root",
			StartUnixNano: int64(i * 100), DurationNano: 100,
		}
	}
	data, meta, err := parquettrace.WriteBlock(rows, parquettrace.DefaultBloom)
	if err != nil { t.Fatal(err) }
	meta.Path = "traces/blk.parquet"
	m := &store.MemStore{Objects: map[string][]byte{"traces/blk.parquet": data, "traces/blk.meta.json": mustMeta(t, meta)}}
	s := &store.Store{OS: m, Prefix: "traces/"}

	mux := http.NewServeMux()
	New(s).Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Test ?limit=5 should return at most 5
	resp, err := http.Get(srv.URL + `/api/search?limit=5`)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Fatalf("status %d", resp.StatusCode) }
	var out struct{ Traces []struct{ TraceID string } }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { t.Fatal(err) }
	if len(out.Traces) > 5 {
		t.Fatalf("expected at most 5 traces with limit=5, got %d", len(out.Traces))
	}
}

func mustMeta(t *testing.T, meta parquettrace.BlockMeta) []byte {
	t.Helper()
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
