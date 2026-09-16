// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parquettrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func marshalComplex(v pcommon.Value) []byte {
	b, err := json.Marshal(v.AsRaw())
	if err != nil {
		return []byte("null")
	}
	return b
}

func unmarshalComplexInto(m pcommon.Map, key string, raw []byte) {
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return
	}
	_ = m.PutEmpty(key).FromRaw(out)
}
