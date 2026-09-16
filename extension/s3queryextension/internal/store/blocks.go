// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"

import (
	"bytes"
	"context"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Store struct {
	OS     ObjectStore
	Prefix string
}

func (s *Store) FindBlocks(ctx context.Context, startNano, endNano int64, service string) ([]parquettrace.BlockMeta, error) {
	keys, err := s.OS.List(ctx, s.Prefix)
	if err != nil {
		return nil, err
	}
	var out []parquettrace.BlockMeta
	for _, k := range keys {
		if !strings.HasSuffix(k, ".meta.json") {
			continue
		}
		raw, err := s.OS.Get(ctx, k)
		if err != nil {
			continue // availability over completeness
		}
		m, err := parquettrace.UnmarshalBlockMeta(raw)
		if err != nil {
			continue
		}
		if m.MaxTimeUnixNano < startNano || m.MinTimeUnixNano > endNano {
			continue
		}
		if service != "" && !contains(m.Services, service) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func (s *Store) TraceByID(ctx context.Context, traceID []byte, startNano, endNano int64) (ptrace.Traces, bool, error) {
	blocks, err := s.FindBlocks(ctx, startNano, endNano, "")
	if err != nil {
		return ptrace.NewTraces(), false, err
	}
	var matched []parquettrace.SpanRow
	for _, b := range blocks {
		data, err := s.OS.Get(ctx, b.Path)
		if err != nil {
			continue
		}
		rows, err := parquettrace.ReadBlock(data)
		if err != nil {
			continue
		}
		for _, r := range rows {
			if bytes.Equal(r.TraceID, traceID) {
				matched = append(matched, r)
			}
		}
	}
	if len(matched) == 0 {
		return ptrace.NewTraces(), false, nil
	}
	return parquettrace.RowsToTraces(matched), true, nil
}
