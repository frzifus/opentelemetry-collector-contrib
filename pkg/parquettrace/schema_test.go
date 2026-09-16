// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace

import "testing"

func TestSchemaConstants(t *testing.T) {
	if SchemaVersion != "1" {
		t.Fatalf("SchemaVersion = %q, want 1", SchemaVersion)
	}
	want := []string{"trace_id", "span_id", "parent_span_id", "start_unix_nano"}
	if len(AlwaysPromoted) != len(want) {
		t.Fatalf("AlwaysPromoted = %v, want %v", AlwaysPromoted, want)
	}
	for i, c := range want {
		if AlwaysPromoted[i] != c {
			t.Fatalf("AlwaysPromoted[%d] = %q, want %q", i, AlwaysPromoted[i], c)
		}
	}
}
