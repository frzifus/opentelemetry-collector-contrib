// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

type s3Exporter struct {
	config      *Config
	signalType  string
	uploader    upload.Manager
	logger      *zap.Logger
	marshaler   marshaler
	blockWriter *blockWriter
}

func newS3Exporter(
	config *Config,
	signalType string,
	params exporter.Settings,
) *s3Exporter {
	s3Exporter := &s3Exporter{
		config:     config,
		signalType: signalType,
		logger:     params.Logger,
	}
	return s3Exporter
}

func (e *s3Exporter) getUploadOpts(res pcommon.Resource) *upload.UploadOptions {
	s3Prefix := ""
	s3Bucket := ""
	if s3PrefixKey := e.config.ResourceAttrsToS3.S3Prefix; s3PrefixKey != "" {
		if value, ok := res.Attributes().Get(s3PrefixKey); ok {
			s3Prefix = value.AsString()
		}
	}
	if s3BucketKey := e.config.ResourceAttrsToS3.S3Bucket; s3BucketKey != "" {
		if value, ok := res.Attributes().Get(s3BucketKey); ok {
			s3Bucket = value.AsString()
		}
	}
	uploadOpts := &upload.UploadOptions{
		OverrideBucket: s3Bucket,
		OverridePrefix: s3Prefix,
	}
	return uploadOpts
}

func (e *s3Exporter) start(ctx context.Context, host component.Host) error {
	var m marshaler
	var err error
	if e.config.Encoding != nil {
		if m, err = newMarshalerFromEncoding(e.config.Encoding, e.config.EncodingFileExtension, host, e.logger); err != nil {
			return err
		}
	} else {
		if m, err = newMarshaler(e.config.MarshalerName, e.logger); err != nil {
			return fmt.Errorf("unknown marshaler %q", e.config.MarshalerName)
		}
	}

	e.marshaler = m

	up, err := newUploadManager(ctx, e.config, e.logger, e.signalType, m.format(), m.compressed())
	if err != nil {
		return err
	}
	e.uploader = up

	// Construct blockWriter when parquet is configured (traces only)
	if pc := e.config.Parquet.Get(); pc != nil && e.signalType == "traces" {
		var s3PartitionTimeLocation *time.Location
		if e.config.S3Uploader.S3PartitionTimezone != "" {
			s3PartitionTimeLocation, err = time.LoadLocation(e.config.S3Uploader.S3PartitionTimezone)
			if err != nil {
				return fmt.Errorf("invalid S3 partition timezone: %w", err)
			}
		} else {
			s3PartitionTimeLocation = time.Local
		}

		pkb := &upload.PartitionKeyBuilder{
			PartitionBasePrefix:   e.config.S3Uploader.S3BasePrefix,
			PartitionPrefix:       e.config.S3Uploader.S3Prefix,
			PartitionFormat:       e.config.S3Uploader.S3PartitionFormat,
			PartitionTimeLocation: s3PartitionTimeLocation,
		}

		keyPrefix := func(ts time.Time) string {
			return pkb.BucketKeyPrefix(ts, "")
		}

		e.blockWriter = newBlockWriter(*pc, up, keyPrefix, e.logger)
	}

	return nil
}

func (*s3Exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *s3Exporter) shutdown(ctx context.Context) error {
	if e.blockWriter != nil {
		return e.blockWriter.Shutdown(ctx)
	}
	return nil
}

func (e *s3Exporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	buf, err := e.marshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(md.ResourceMetrics().At(0).Resource())
	return e.uploader.Upload(ctx, buf, uploadOpts)
}

func (e *s3Exporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	buf, err := e.marshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(logs.ResourceLogs().At(0).Resource())

	return e.uploader.Upload(ctx, buf, uploadOpts)
}

func (e *s3Exporter) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	if e.blockWriter != nil {
		return e.blockWriter.Add(ctx, traces)
	}

	buf, err := e.marshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(traces.ResourceSpans().At(0).Resource())

	return e.uploader.Upload(ctx, buf, uploadOpts)
}
