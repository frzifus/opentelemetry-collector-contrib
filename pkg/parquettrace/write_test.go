// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace

import "testing"

func TestWriteReadBlockRoundTrip(t *testing.T) {
	rows := []SpanRow{
		{TraceID: []byte{1}, SpanID: []byte{2}, ServiceName: "a", SpanName: "s1", StartUnixNano: 10, DurationNano: 5, AttrsInt: map[string]int64{"code": 200}},
		{TraceID: []byte{3}, SpanID: []byte{4}, ServiceName: "b", SpanName: "s2", StartUnixNano: 30, DurationNano: 7},
	}
	data, meta, err := WriteBlock(rows, DefaultBloom)
	if err != nil {
		t.Fatal(err)
	}
	if meta.RowCount != 2 || meta.MinTimeUnixNano != 10 || meta.MaxTimeUnixNano != 30 {
		t.Fatalf("meta wrong: %+v", meta)
	}
	if len(meta.Services) != 2 {
		t.Fatalf("services = %v, want 2", meta.Services)
	}
	got, err := ReadBlock(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SpanName != "s1" {
		t.Fatalf("read back wrong: %+v", got)
	}
	// Verify attribute map round-trip
	if got[0].AttrsInt["code"] != 200 {
		t.Fatalf("attribute not preserved: got %v, want 200", got[0].AttrsInt["code"])
	}
}

func TestMetaKeyFor(t *testing.T) {
	if got := MetaKeyFor("traces/hour=10/blk1.parquet"); got != "traces/hour=10/blk1.meta.json" {
		t.Fatalf("MetaKeyFor = %q", got)
	}
}
