// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"encoding/json"
	"testing"

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
