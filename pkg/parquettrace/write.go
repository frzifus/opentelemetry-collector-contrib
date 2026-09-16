// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"

import (
	"bytes"
	"sort"

	"github.com/parquet-go/parquet-go"
)

// WriteBlock encodes rows to Parquet (bloom filters on the named columns) and
// returns the bytes plus computed BlockMeta (BlockID/Path left blank for the caller).
func WriteBlock(rows []SpanRow, bloom []string) ([]byte, BlockMeta, error) {
	var buf bytes.Buffer
	opts := []parquet.WriterOption{}
	for _, col := range bloom {
		opts = append(opts, parquet.BloomFilters(parquet.SplitBlockFilter(10, col)))
	}
	w := parquet.NewGenericWriter[SpanRow](&buf, opts...)
	if _, err := w.Write(rows); err != nil {
		return nil, BlockMeta{}, err
	}
	if err := w.Close(); err != nil {
		return nil, BlockMeta{}, err
	}
	meta := computeMeta(rows, buf.Len())
	return buf.Bytes(), meta, nil
}

func ReadBlock(data []byte) ([]SpanRow, error) {
	rows, err := parquet.Read[SpanRow](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func computeMeta(rows []SpanRow, size int) BlockMeta {
	m := BlockMeta{
		SchemaVersion:   SchemaVersion,
		RowCount:        len(rows),
		SizeBytes:       size,
		PromotedColumns: append(append([]string{}, AlwaysPromoted...), DefaultPromoted...),
	}
	svcSet := map[string]struct{}{}
	for i, r := range rows {
		if i == 0 || r.StartUnixNano < m.MinTimeUnixNano {
			m.MinTimeUnixNano = r.StartUnixNano
		}
		if i == 0 || r.StartUnixNano > m.MaxTimeUnixNano {
			m.MaxTimeUnixNano = r.StartUnixNano
		}
		if r.ServiceName != "" {
			svcSet[r.ServiceName] = struct{}{}
		}
	}
	for s := range svcSet {
		m.Services = append(m.Services, s)
	}
	sort.Strings(m.Services)
	return m
}
