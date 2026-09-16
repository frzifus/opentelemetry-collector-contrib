// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func TestSearchMatchesServiceAndDuration(t *testing.T) {
	rows := []parquettrace.SpanRow{
		{TraceID: []byte{1}, SpanID: []byte{1}, ServiceName: "checkout", SpanName: "slow", StartUnixNano: 10, DurationNano: 200_000_000},
		{TraceID: []byte{2}, SpanID: []byte{2}, ServiceName: "checkout", SpanName: "fast", StartUnixNano: 12, DurationNano: 1_000},
		{TraceID: []byte{3}, SpanID: []byte{3}, ServiceName: "frontend", SpanName: "slow", StartUnixNano: 14, DurationNano: 900_000_000},
	}
	data, meta, err := parquettrace.WriteBlock(rows, parquettrace.DefaultBloom)
	if err != nil { t.Fatal(err) }
	meta.Path = "t/blk.parquet"
	m := &MemStore{Objects: map[string][]byte{"t/blk.parquet": data, "t/blk.meta.json": metaJSON(t, meta)}}
	s := &Store{OS: m, Prefix: "t/"}
	q := traceql.Query{ServiceName: "checkout", MinDurationNano: 100_000_000}
	res, err := s.Search(context.Background(), q, 0, 100, 10)
	if err != nil { t.Fatal(err) }
	if len(res) != 1 || string(res[0].TraceID) != string([]byte{1}) {
		t.Fatalf("search = %+v, want only trace 1", res)
	}
}
