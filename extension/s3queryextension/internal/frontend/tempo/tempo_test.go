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

func mustMeta(t *testing.T, meta parquettrace.BlockMeta) []byte {
	t.Helper()
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
