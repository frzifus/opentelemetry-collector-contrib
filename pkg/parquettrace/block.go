// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"

import (
	"encoding/json"
	"strings"
)

type BlockMeta struct {
	BlockID         string   `json:"block_id"`
	SchemaVersion   string   `json:"schema_version"`
	MinTimeUnixNano int64    `json:"min_time_unix_nano"`
	MaxTimeUnixNano int64    `json:"max_time_unix_nano"`
	RowCount        int      `json:"row_count"`
	SizeBytes       int      `json:"size_bytes"`
	Services        []string `json:"services"`
	PromotedColumns []string `json:"promoted_columns"`
	Path            string   `json:"path"`
}

func UnmarshalBlockMeta(b []byte) (BlockMeta, error) {
	var m BlockMeta
	err := json.Unmarshal(b, &m)
	return m, err
}

func MetaKeyFor(parquetKey string) string {
	return strings.TrimSuffix(parquetKey, ".parquet") + ".meta.json"
}
