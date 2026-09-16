// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

var (
	s3PrefixKey    = "_sourceHost"
	s3BucketKey    = "_sourceBucket"
	overridePrefix = "host"
	overrideBucket = "my-bucket"
	testLogs       = fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"_sourceCategory","value":{"stringValue":"logfile"}},{"key":"%s","value":{"stringValue":"%s"}},{"key":"%s","value":{"stringValue":"%s"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1654257420681895000","body":{"stringValue":"2022-06-03 13:57:00.62739 +0200 CEST m=+14.018296742 log entry14"},"attributes":[{"key":"log.file.path_resolved","value":{"stringValue":"logwriter/data.log"}}]}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.6.1"}]}`, s3PrefixKey, overridePrefix, s3BucketKey, overrideBucket) //nolint:gocritic //sprintfQuotedString for JSON
)

type testWriter struct {
	t            *testing.T
	expectedOpts *upload.UploadOptions
}

func (testWriter *testWriter) Upload(_ context.Context, buf []byte, uploadOpts *upload.UploadOptions) error {
	assert.JSONEq(testWriter.t, testLogs, string(buf))
	assert.Equal(testWriter.t, testWriter.expectedOpts, uploadOpts)
	return nil
}

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs([]byte(testLogs))
	assert.NoError(tb, err, "Can't unmarshal testing the logs data -> %s", err)
	return logs
}

func getLogExporter(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	exporter := &s3Exporter{
		config:    createDefaultConfig().(*Config),
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverridePrefix: ""}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLog(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporter(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

func getLogExporterWithResourceAttrs(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey
	exporter := &s3Exporter{
		config:    config,
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverridePrefix: overridePrefix}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLogWithResourceAttrs(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporterWithResourceAttrs(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

func getLogExporterWithBucketAndPrefixAttrs(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Bucket = s3BucketKey
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey
	exporter := &s3Exporter{
		config:    config,
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverrideBucket: overrideBucket, OverridePrefix: overridePrefix}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLogWithBucketAndPrefixAttrs(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporterWithBucketAndPrefixAttrs(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

// fakeUploadManager tracks all uploads for test assertions
type fakeUploadManager struct {
	uploads []fakeUpload
}

type fakeUpload struct {
	key  string
	data []byte
}

func (f *fakeUploadManager) Upload(_ context.Context, data []byte, opts *upload.UploadOptions) error {
	key := ""
	if opts != nil && opts.OverrideKey != "" {
		key = opts.OverrideKey
	}
	f.uploads = append(f.uploads, fakeUpload{key: key, data: data})
	return nil
}

func TestConsumeTracesUsesBlockWriterWhenParquetSet(t *testing.T) {
	// Build an s3Exporter with cfg.Parquet set and a fake upload.Manager
	cfg := createDefaultConfig().(*Config)
	cfg.S3Uploader.S3Bucket = "test-bucket"
	cfg.Parquet = configoptional.Some(ParquetConfig{
		TargetBlockSize: 1, // force flush on every Add
		MaxBlockAge:     60 * time.Second,
	})

	fakeUp := &fakeUploadManager{}
	params := exporter.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}
	s3Exp := newS3Exporter(cfg, "traces", params)

	// Manually set the uploader and blockWriter (bypass start() to avoid AWS client creation)
	s3Exp.uploader = fakeUp
	s3Exp.blockWriter = newBlockWriter(
		*cfg.Parquet.Get(),
		fakeUp,
		func(time.Time) string { return "traces/hour=10" },
		zap.NewNop(),
	)

	// Consume traces
	ctx := context.Background()
	traces := sampleTraces()
	err := s3Exp.ConsumeTraces(ctx, traces)
	require.NoError(t, err)

	// Shutdown to flush
	err = s3Exp.blockWriter.Shutdown(ctx)
	require.NoError(t, err)

	// Assert that a .parquet upload occurred
	parquetUploaded := false
	for _, up := range fakeUp.uploads {
		if strings.HasSuffix(up.key, ".parquet") {
			parquetUploaded = true
			break
		}
	}
	assert.True(t, parquetUploaded, "expected a .parquet file upload, got keys: %v",
		func() []string {
			keys := make([]string, len(fakeUp.uploads))
			for i, u := range fakeUp.uploads {
				keys[i] = u.key
			}
			return keys
		}())
}
