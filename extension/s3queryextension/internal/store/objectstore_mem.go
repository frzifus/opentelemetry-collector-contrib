// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type MemStore struct{ Objects map[string][]byte }

func (m *MemStore) List(_ context.Context, prefix string) ([]string, error) {
	var out []string
	for k := range m.Objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (m *MemStore) Get(_ context.Context, key string) ([]byte, error) {
	b, ok := m.Objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return b, nil
}
