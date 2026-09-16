// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"

import (
	"context"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
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
