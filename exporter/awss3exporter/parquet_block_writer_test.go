// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

type capUpload struct {
	keys []string
	data map[string][]byte
}

func (c *capUpload) Upload(_ context.Context, data []byte, opts *upload.UploadOptions) error {
	if c.data == nil {
		c.data = map[string][]byte{}
	}
	c.keys = append(c.keys, opts.OverrideKey)
	c.data[opts.OverrideKey] = append([]byte{}, data...)
	return nil
}

func TestBlockWriterFlushesBlockAndMeta(t *testing.T) {
	cap := &capUpload{}
	bw := newBlockWriter(
		ParquetConfig{
			TargetBlockSize:    1, // size=1 forces flush every Add
			MaxBlockAge:        time.Hour,
			PromotedAttributes: parquettrace.DefaultPromoted,
			BloomAttributes:    parquettrace.DefaultBloom,
		},
		cap,
		func(time.Time) string { return "traces/hour=10" },
		zap.NewNop(),
	)
	defer func() {
		if err := bw.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()
	if err := bw.Add(context.Background(), sampleTraces()); err != nil {
		t.Fatal(err)
	}
	var parquetKey, metaKey string
	for _, k := range cap.keys {
		if strings.HasSuffix(k, ".parquet") {
			parquetKey = k
		}
		if strings.HasSuffix(k, ".meta.json") {
			metaKey = k
		}
	}
	if parquetKey == "" || metaKey == "" {
		t.Fatalf("expected both keys, got %v", cap.keys)
	}
	if parquettrace.MetaKeyFor(parquetKey) != metaKey {
		t.Fatalf("meta key %q does not match parquet key %q", metaKey, parquetKey)
	}
	meta, err := parquettrace.UnmarshalBlockMeta(cap.data[metaKey])
	if err != nil || meta.RowCount == 0 || meta.Path != parquetKey {
		t.Fatalf("bad meta: %+v err=%v", meta, err)
	}
}

func sampleTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	sp := ss.Spans().AppendEmpty()
	sp.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	sp.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	sp.SetName("test-span")
	sp.SetKind(ptrace.SpanKindServer)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return td
}
