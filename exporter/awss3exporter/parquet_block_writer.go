// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"encoding/json"
	"path"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

type blockWriter struct {
	cfg       ParquetConfig
	up        upload.Manager
	keyPrefix func(time.Time) string
	logger    *zap.Logger

	mu       sync.Mutex
	rows     []parquettrace.SpanRow
	estBytes int64
	ticker   *time.Ticker
	done     chan struct{}
}

func newBlockWriter(cfg ParquetConfig, up upload.Manager, keyPrefix func(time.Time) string, logger *zap.Logger) *blockWriter {
	b := &blockWriter{
		cfg:       cfg,
		up:        up,
		keyPrefix: keyPrefix,
		logger:    logger,
		done:      make(chan struct{}),
	}
	b.ticker = time.NewTicker(cfg.MaxBlockAge)
	go b.ageLoop()
	return b
}

func (b *blockWriter) ageLoop() {
	for {
		select {
		case <-b.done:
			return
		case <-b.ticker.C:
			b.mu.Lock()
			if len(b.rows) > 0 {
				if err := b.flushLocked(context.Background()); err != nil {
					b.logger.Error("age flush failed", zap.Error(err))
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *blockWriter) Add(ctx context.Context, td ptrace.Traces) error {
	rows := parquettrace.TracesToRows(td)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rows = append(b.rows, rows...)
	b.estBytes += estimateSize(rows)
	if b.estBytes >= b.cfg.TargetBlockSize {
		return b.flushLocked(ctx)
	}
	return nil
}

func (b *blockWriter) flushLocked(ctx context.Context) error {
	if len(b.rows) == 0 {
		return nil
	}
	data, meta, err := parquettrace.WriteBlock(b.rows, b.cfg.BloomAttributes)
	if err != nil {
		return err
	}
	blockID := upload.GenerateUUIDv7()
	key := path.Join(b.keyPrefix(time.Now()), blockID+".parquet")
	meta.BlockID = blockID
	meta.Path = key
	metaBytes, err := marshalMeta(meta)
	if err != nil {
		return err
	}
	if err := b.up.Upload(ctx, data, &upload.UploadOptions{OverrideKey: key}); err != nil {
		return err
	}
	if err := b.up.Upload(ctx, metaBytes, &upload.UploadOptions{OverrideKey: parquettrace.MetaKeyFor(key)}); err != nil {
		return err
	}
	b.rows = nil
	b.estBytes = 0
	return nil
}

func (b *blockWriter) Shutdown(ctx context.Context) error {
	close(b.done)
	b.ticker.Stop()
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked(ctx)
}

func estimateSize(rows []parquettrace.SpanRow) int64 {
	const fixedOverhead = 100 // rough per-row overhead
	size := int64(0)
	for _, r := range rows {
		size += fixedOverhead
		size += int64(len(r.SpanName))
		size += int64(len(r.ServiceName))
		for k, v := range r.AttrsStr {
			size += int64(len(k) + len(v))
		}
	}
	return size
}

func marshalMeta(meta parquettrace.BlockMeta) ([]byte, error) {
	return json.Marshal(meta)
}
