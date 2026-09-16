// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"testing"
)

func TestMemStoreListAndGet(t *testing.T) {
	m := &MemStore{Objects: map[string][]byte{
		"traces/hour=10/a.meta.json": []byte("A"),
		"traces/hour=10/a.parquet":   []byte("P"),
		"other/x":                    []byte("X"),
	}}
	keys, err := m.List(context.Background(), "traces/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("List returned %v, want 2 under traces/", keys)
	}
	b, err := m.Get(context.Background(), "traces/hour=10/a.meta.json")
	if err != nil || string(b) != "A" {
		t.Fatalf("Get = %q err=%v", b, err)
	}
}
