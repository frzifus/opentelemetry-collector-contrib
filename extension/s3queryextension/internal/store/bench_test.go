// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"encoding/hex"
	"os"
	"sync"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
)

// countingStore wraps an ObjectStore and counts List and Get calls.
type countingStore struct {
	inner ObjectStore
	mu    sync.Mutex
	Lists int
	Gets  int
}

func (c *countingStore) List(ctx context.Context, prefix string) ([]string, error) {
	c.mu.Lock()
	c.Lists++
	c.mu.Unlock()
	return c.inner.List(ctx, prefix)
}

func (c *countingStore) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	c.Gets++
	c.mu.Unlock()
	return c.inner.Get(ctx, key)
}

func (c *countingStore) reset() {
	c.mu.Lock()
	c.Lists = 0
	c.Gets = 0
	c.mu.Unlock()
}

func (c *countingStore) counts() (lists, gets int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Lists, c.Gets
}

// BenchmarkQueryTraceByID measures TraceByID latency and S3 request counts against a real S3 bucket.
// Set AWS_S3_BENCH_BUCKET to run; skips if unset.
func BenchmarkQueryTraceByID(b *testing.B) {
	bucket := os.Getenv("AWS_S3_BENCH_BUCKET")
	if bucket == "" {
		b.Skip("set AWS_S3_BENCH_BUCKET to run")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	endpoint := os.Getenv("AWS_S3_BENCH_ENDPOINT")
	prefix := os.Getenv("AWS_S3_BENCH_PREFIX")
	if prefix == "" {
		prefix = "traces/"
	}

	// Default trace ID (user can override with AWS_S3_BENCH_TRACEID hex)
	traceIDHex := os.Getenv("AWS_S3_BENCH_TRACEID")
	if traceIDHex == "" {
		// Default: all zeros (user should set a real trace ID from their bucket)
		traceIDHex = "00000000000000000000000000000000"
	}
	traceID, err := hex.DecodeString(traceIDHex)
	if err != nil {
		b.Fatalf("invalid AWS_S3_BENCH_TRACEID: %v", err)
	}

	ctx := context.Background()
	s3OS, err := NewS3Store(ctx, S3Config{
		Region:         region,
		Bucket:         bucket,
		Endpoint:       endpoint,
		ForcePathStyle: endpoint != "",
		DisableSSL:     false,
	})
	if err != nil {
		b.Fatalf("NewS3Store: %v", err)
	}

	counting := &countingStore{inner: s3OS}
	store := &Store{OS: counting, Prefix: prefix}

	// Time window: last 24 hours (adjust if needed)
	now := int64(1726502400000000000) // 2024-09-16 12:00:00 UTC (example; user can parameterize)
	startNano := now - 24*3600*1e9
	endNano := now

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counting.reset()
		_, found, err := store.TraceByID(ctx, traceID, startNano, endNano)
		if err != nil {
			b.Fatalf("TraceByID: %v", err)
		}
		if !found {
			b.Logf("trace %s not found (expected if bucket empty or time window wrong)", traceIDHex)
		}
		lists, gets := counting.counts()
		b.ReportMetric(float64(lists), "lists/op")
		b.ReportMetric(float64(gets), "gets/op")
	}
}

// BenchmarkQuerySearch measures Search latency and S3 request counts against a real S3 bucket.
// Set AWS_S3_BENCH_BUCKET to run; skips if unset.
func BenchmarkQuerySearch(b *testing.B) {
	bucket := os.Getenv("AWS_S3_BENCH_BUCKET")
	if bucket == "" {
		b.Skip("set AWS_S3_BENCH_BUCKET to run")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	endpoint := os.Getenv("AWS_S3_BENCH_ENDPOINT")
	prefix := os.Getenv("AWS_S3_BENCH_PREFIX")
	if prefix == "" {
		prefix = "traces/"
	}

	// Default service name (user can override with AWS_S3_BENCH_SERVICE)
	service := os.Getenv("AWS_S3_BENCH_SERVICE")
	if service == "" {
		service = "frontend" // example default
	}

	ctx := context.Background()
	s3OS, err := NewS3Store(ctx, S3Config{
		Region:         region,
		Bucket:         bucket,
		Endpoint:       endpoint,
		ForcePathStyle: endpoint != "",
		DisableSSL:     false,
	})
	if err != nil {
		b.Fatalf("NewS3Store: %v", err)
	}

	counting := &countingStore{inner: s3OS}
	store := &Store{OS: counting, Prefix: prefix}

	// Simple query: service.name = <service>
	q := traceql.Query{ServiceName: service}

	// Time window: last 24 hours
	now := int64(1726502400000000000) // 2024-09-16 12:00:00 UTC
	startNano := now - 24*3600*1e9
	endNano := now
	limit := 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counting.reset()
		results, err := store.Search(ctx, q, startNano, endNano, limit)
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			b.Logf("search returned 0 results (expected if bucket empty or no matching service)")
		}
		lists, gets := counting.counts()
		b.ReportMetric(float64(lists), "lists/op")
		b.ReportMetric(float64(gets), "gets/op")
	}
}
