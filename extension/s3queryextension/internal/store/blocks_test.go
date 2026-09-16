// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func metaJSON(t *testing.T, m parquettrace.BlockMeta) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil { t.Fatal(err) }
	return b
}

func TestFindBlocksPrunesByTimeAndService(t *testing.T) {
	m := &MemStore{Objects: map[string][]byte{
		"t/a.meta.json": metaJSON(t, parquettrace.BlockMeta{MinTimeUnixNano: 0, MaxTimeUnixNano: 100, Services: []string{"checkout"}, Path: "t/a.parquet"}),
		"t/b.meta.json": metaJSON(t, parquettrace.BlockMeta{MinTimeUnixNano: 200, MaxTimeUnixNano: 300, Services: []string{"frontend"}, Path: "t/b.parquet"}),
	}}
	s := &Store{OS: m, Prefix: "t/"}
	got, err := s.FindBlocks(context.Background(), 50, 150, "checkout")
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Path != "t/a.parquet" {
		t.Fatalf("FindBlocks = %+v, want only t/a.parquet", got)
	}
}

func TestSearchLimitZero(t *testing.T) {
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
	meta.Path = "t/blk.parquet"
	m := &MemStore{Objects: map[string][]byte{"t/blk.parquet": data, "t/blk.meta.json": metaJSON(t, meta)}}
	s := &Store{OS: m, Prefix: "t/"}

	// Search with limit=0 should be bounded by internal cap
	res, err := s.Search(context.Background(), traceql.Query{}, 0, 1<<62-1, 0)
	if err != nil { t.Fatal(err) }
	if len(res) > defaultSearchCap {
		t.Fatalf("expected at most %d results with limit=0, got %d", defaultSearchCap, len(res))
	}
}

func TestSearchLimitTwo(t *testing.T) {
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
	meta.Path = "t/blk.parquet"
	m := &MemStore{Objects: map[string][]byte{"t/blk.parquet": data, "t/blk.meta.json": metaJSON(t, meta)}}
	s := &Store{OS: m, Prefix: "t/"}

	// Search with limit=2 should return at most 2
	res, err := s.Search(context.Background(), traceql.Query{}, 0, 1<<62-1, 2)
	if err != nil { t.Fatal(err) }
	if len(res) != 2 {
		t.Fatalf("expected exactly 2 results with limit=2, got %d", len(res))
	}
}
