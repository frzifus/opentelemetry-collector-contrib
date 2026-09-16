// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func TestTraceByIDCollectsMatchingSpans(t *testing.T) {
	rows := []parquettrace.SpanRow{
		{TraceID: []byte{1}, SpanID: []byte{1}, ServiceName: "a", SpanName: "s1", StartUnixNano: 10},
		{TraceID: []byte{2}, SpanID: []byte{2}, ServiceName: "a", SpanName: "s2", StartUnixNano: 12},
		{TraceID: []byte{1}, SpanID: []byte{3}, ServiceName: "b", SpanName: "s3", StartUnixNano: 14},
	}
	data, meta, err := parquettrace.WriteBlock(rows, parquettrace.DefaultBloom)
	if err != nil { t.Fatal(err) }
	meta.Path = "t/blk.parquet"
	m := &MemStore{Objects: map[string][]byte{
		"t/blk.parquet":   data,
		"t/blk.meta.json": metaJSON(t, meta),
	}}
	s := &Store{OS: m, Prefix: "t/"}
	td, found, err := s.TraceByID(context.Background(), []byte{1}, 0, 100)
	if err != nil || !found { t.Fatalf("found=%v err=%v", found, err) }
	total := 0
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			total += sss.At(j).Spans().Len()
		}
	}
	if total != 2 {
		t.Fatalf("collected %d spans, want 2 (trace id 1)", total)
	}
	_ = bytes.Equal
}
