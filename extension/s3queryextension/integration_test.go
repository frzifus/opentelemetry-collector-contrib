// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package s3queryextension

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func TestRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	// Set AWS credentials for MinIO
	t.Setenv("AWS_ACCESS_KEY_ID", "minioadmin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "minioadmin")
	t.Setenv("AWS_REGION", "us-east-1")

	ctx := context.Background()

	// Start MinIO container
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "quay.io/minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForListeningPort("9000/tcp"),
		},
		Started: true,
	}

	ctr, err := testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, ctr.Terminate(ctx))
	}()

	host, err := ctr.Host(ctx)
	require.NoError(t, err)

	port, err := ctr.MappedPort(ctx, "9000")
	require.NoError(t, err)

	minioEndpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	// Create S3 client to create bucket
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	require.NoError(t, err)

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(minioEndpoint)
		o.UsePathStyle = true
		o.EndpointOptions.DisableHTTPS = true
	})

	// Create bucket
	_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("traces"),
	})
	require.NoError(t, err)

	// Configure awss3exporter with Parquet mode
	factory := awss3exporter.NewFactory()
	exporterCfg := factory.CreateDefaultConfig().(*awss3exporter.Config)
	exporterCfg.S3Uploader.Endpoint = minioEndpoint
	exporterCfg.S3Uploader.Region = "us-east-1"
	exporterCfg.S3Uploader.S3Bucket = "traces"
	exporterCfg.S3Uploader.S3Prefix = "traces"
	exporterCfg.S3Uploader.S3ForcePathStyle = true
	exporterCfg.S3Uploader.DisableSSL = true
	exporterCfg.Parquet = configoptional.Some(awss3exporter.ParquetConfig{
		TargetBlockSize:    1 << 20, // 1MB
		MaxBlockAge:        time.Second,
		PromotedAttributes: parquettrace.DefaultPromoted,
		BloomAttributes:    parquettrace.DefaultBloom,
	})

	// Create exporter
	set := exporter.Settings{
		ID:                component.MustNewIDWithName("awss3", "test"),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
	tracesExporter, err := factory.CreateTraces(ctx, set, exporterCfg)
	require.NoError(t, err)
	require.NoError(t, tracesExporter.Start(ctx, nil))

	// Build known traces: 2 traces, 5 spans total
	traces := buildTestTraces()

	// Write traces via exporter
	err = tracesExporter.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// Shutdown to flush
	require.NoError(t, tracesExporter.Shutdown(ctx))

	// Give it a moment to finish writing
	time.Sleep(2 * time.Second)

	// Create store for reading
	os, err := store.NewS3Store(ctx, store.S3Config{
		Region:         "us-east-1",
		Bucket:         "traces",
		Endpoint:       minioEndpoint,
		ForcePathStyle: true,
		DisableSSL:     true,
	})
	require.NoError(t, err)

	st := &store.Store{
		OS:     os,
		Prefix: "traces/",
	}

	// Query by known trace ID
	traceID1 := mustDecodeHex("11111111111111111111111111111111")
	traceID2 := mustDecodeHex("22222222222222222222222222222222")

	// Wait for blocks to be available with retry
	var blocks []parquettrace.BlockMeta
	for i := 0; i < 10; i++ {
		blocks, err = st.FindBlocks(ctx, 0, time.Now().UnixNano()+int64(time.Hour), "")
		if err == nil && len(blocks) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err)
	require.NotEmpty(t, blocks, "Expected at least one block to be written")

	// Test TraceByID for first trace (3 spans)
	td1, found, err := st.TraceByID(ctx, traceID1, 0, time.Now().UnixNano()+int64(time.Hour))
	require.NoError(t, err)
	require.True(t, found, "Expected to find trace ID 1")
	require.Equal(t, 3, td1.SpanCount(), "Expected 3 spans for trace ID 1")

	// Test TraceByID for second trace (2 spans)
	td2, found, err := st.TraceByID(ctx, traceID2, 0, time.Now().UnixNano()+int64(time.Hour))
	require.NoError(t, err)
	require.True(t, found, "Expected to find trace ID 2")
	require.Equal(t, 2, td2.SpanCount(), "Expected 2 spans for trace ID 2")

	// Test Search by service name "checkout"
	results, err := st.Search(ctx, traceql.Query{ServiceName: "checkout"}, 0, time.Now().UnixNano()+int64(time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, results, 2, "Expected to find 2 traces with service 'checkout'")

	// Verify trace IDs in results
	foundTrace1 := false
	foundTrace2 := false
	for _, r := range results {
		if string(r.TraceID) == string(traceID1) {
			foundTrace1 = true
			require.Equal(t, "checkout", r.RootServiceName)
		}
		if string(r.TraceID) == string(traceID2) {
			foundTrace2 = true
			require.Equal(t, "checkout", r.RootServiceName)
		}
	}
	require.True(t, foundTrace1, "Expected to find trace ID 1 in search results")
	require.True(t, foundTrace2, "Expected to find trace ID 2 in search results")
}

// buildTestTraces creates 2 traces with 5 total spans
func buildTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")

	ss := rs.ScopeSpans().AppendEmpty()

	// Trace 1: 3 spans
	traceID1 := mustDecodeHex("11111111111111111111111111111111")
	span1 := ss.Spans().AppendEmpty()
	span1.SetTraceID(pcommon.TraceID(traceID1))
	span1.SetSpanID(pcommon.SpanID(mustDecodeHex("1111111111111111")))
	span1.SetName("checkout-span-1")
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Hour)))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1*time.Hour + 100*time.Millisecond)))

	span2 := ss.Spans().AppendEmpty()
	span2.SetTraceID(pcommon.TraceID(traceID1))
	span2.SetSpanID(pcommon.SpanID(mustDecodeHex("2222222222222222")))
	span2.SetName("checkout-span-2")
	span2.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Hour)))
	span2.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1*time.Hour + 50*time.Millisecond)))

	span3 := ss.Spans().AppendEmpty()
	span3.SetTraceID(pcommon.TraceID(traceID1))
	span3.SetSpanID(pcommon.SpanID(mustDecodeHex("3333333333333333")))
	span3.SetName("checkout-span-3")
	span3.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Hour)))
	span3.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1*time.Hour + 75*time.Millisecond)))

	// Trace 2: 2 spans
	traceID2 := mustDecodeHex("22222222222222222222222222222222")
	span4 := ss.Spans().AppendEmpty()
	span4.SetTraceID(pcommon.TraceID(traceID2))
	span4.SetSpanID(pcommon.SpanID(mustDecodeHex("4444444444444444")))
	span4.SetName("checkout-span-4")
	span4.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Hour)))
	span4.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1*time.Hour + 200*time.Millisecond)))

	span5 := ss.Spans().AppendEmpty()
	span5.SetTraceID(pcommon.TraceID(traceID2))
	span5.SetSpanID(pcommon.SpanID(mustDecodeHex("5555555555555555")))
	span5.SetName("checkout-span-5")
	span5.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Hour)))
	span5.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1*time.Hour + 150*time.Millisecond)))

	return traces
}

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
