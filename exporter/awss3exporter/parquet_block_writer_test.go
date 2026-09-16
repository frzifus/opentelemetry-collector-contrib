// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

type capUpload struct {
	mu   sync.Mutex
	keys []string
	data map[string][]byte
}

func (c *capUpload) Upload(_ context.Context, data []byte, opts *upload.UploadOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = map[string][]byte{}
	}
	c.keys = append(c.keys, opts.OverrideKey)
	c.data[opts.OverrideKey] = append([]byte{}, data...)
	return nil
}

func (c *capUpload) getKeys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.keys...)
}

func (c *capUpload) getData(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key]
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
	keys := cap.getKeys()
	var parquetKey, metaKey string
	for _, k := range keys {
		if strings.HasSuffix(k, ".parquet") {
			parquetKey = k
		}
		if strings.HasSuffix(k, ".meta.json") {
			metaKey = k
		}
	}
	if parquetKey == "" || metaKey == "" {
		t.Fatalf("expected both keys, got %v", keys)
	}
	if parquettrace.MetaKeyFor(parquetKey) != metaKey {
		t.Fatalf("meta key %q does not match parquet key %q", metaKey, parquetKey)
	}
	meta, err := parquettrace.UnmarshalBlockMeta(cap.getData(metaKey))
	if err != nil || meta.RowCount == 0 || meta.Path != parquetKey {
		t.Fatalf("bad meta: %+v err=%v", meta, err)
	}
}

func TestBlockWriterShutdownIdempotent(t *testing.T) {
	cap := &capUpload{}
	bw := newBlockWriter(
		ParquetConfig{
			TargetBlockSize:    1024 * 1024,
			MaxBlockAge:        time.Hour,
			PromotedAttributes: parquettrace.DefaultPromoted,
			BloomAttributes:    parquettrace.DefaultBloom,
		},
		cap,
		func(time.Time) string { return "traces/hour=10" },
		zap.NewNop(),
	)
	ctx := context.Background()
	// First shutdown should succeed
	if err := bw.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown failed: %v", err)
	}
	// Second shutdown should not panic and should return nil
	if err := bw.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown failed: %v", err)
	}
}

func TestBlockWriterAgeBasedFlush(t *testing.T) {
	cap := &capUpload{}
	bw := newBlockWriter(
		ParquetConfig{
			TargetBlockSize:    1024 * 1024 * 1024, // very large to avoid size-based flush
			MaxBlockAge:        50 * time.Millisecond,
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
	// Add traces but don't trigger size-based flush
	if err := bw.Add(context.Background(), sampleTraces()); err != nil {
		t.Fatal(err)
	}
	// Initially no uploads should have happened
	if len(cap.getKeys()) > 0 {
		t.Fatalf("expected no uploads yet, got %d", len(cap.getKeys()))
	}
	// Wait for the age ticker to fire
	time.Sleep(100 * time.Millisecond)
	// Now we should have a flush from the age ticker
	keys := cap.getKeys()
	if len(keys) == 0 {
		t.Fatal("expected age-based flush to upload blocks")
	}
	var parquetKey string
	for _, k := range keys {
		if strings.HasSuffix(k, ".parquet") {
			parquetKey = k
		}
	}
	if parquetKey == "" {
		t.Fatalf("expected .parquet key after age flush, got %v", keys)
	}
}

type failingUpload struct {
	mu        sync.Mutex
	failCount int
	delegate  *capUpload
}

func (f *failingUpload) Upload(ctx context.Context, data []byte, opts *upload.UploadOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCount > 0 {
		f.failCount--
		return errors.New("simulated upload failure")
	}
	return f.delegate.Upload(ctx, data, opts)
}

func TestBlockWriterRollbackOnFlushError(t *testing.T) {
	cap := &capUpload{}
	failing := &failingUpload{failCount: 1, delegate: cap}
	bw := newBlockWriter(
		ParquetConfig{
			TargetBlockSize:    1, // size=1 forces flush on every Add
			MaxBlockAge:        time.Hour,
			PromotedAttributes: parquettrace.DefaultPromoted,
			BloomAttributes:    parquettrace.DefaultBloom,
		},
		failing,
		func(time.Time) string { return "traces/hour=10" },
		zap.NewNop(),
	)
	defer func() {
		if err := bw.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	// First Add should trigger flush which fails
	td := sampleTraces()
	err := bw.Add(context.Background(), td)
	if err == nil {
		t.Fatal("expected error from first Add, got nil")
	}

	// Second Add should succeed (failCount is now 0)
	err = bw.Add(context.Background(), td)
	if err != nil {
		t.Fatalf("expected second Add to succeed, got error: %v", err)
	}

	// Verify only one parquet file was written (no duplication)
	keys := cap.getKeys()
	parquetCount := 0
	var parquetKey string
	for _, k := range keys {
		if strings.HasSuffix(k, ".parquet") {
			parquetCount++
			parquetKey = k
		}
	}
	if parquetCount != 1 {
		t.Fatalf("expected exactly 1 parquet file, got %d", parquetCount)
	}

	// Verify the parquet file contains the expected number of rows (not duplicated)
	// Read the block and check row count
	meta, err := parquettrace.UnmarshalBlockMeta(cap.getData(parquettrace.MetaKeyFor(parquetKey)))
	if err != nil {
		t.Fatalf("failed to unmarshal meta: %v", err)
	}
	expectedRows := len(parquettrace.TracesToRows(td))
	if meta.RowCount != expectedRows {
		t.Fatalf("expected %d rows, got %d (possible duplication)", expectedRows, meta.RowCount)
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
