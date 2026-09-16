// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquettrace"
)

func matchRow(r parquettrace.SpanRow, q traceql.Query) bool {
	if q.ServiceName != "" && r.ServiceName != q.ServiceName {
		return false
	}
	if q.SpanName != "" && r.SpanName != q.SpanName {
		return false
	}
	if q.MinDurationNano > 0 && r.DurationNano < q.MinDurationNano {
		return false
	}
	if q.StatusCode != nil && r.StatusCode != *q.StatusCode {
		return false
	}
	for _, c := range q.Attrs {
		if !matchAttr(r, c) {
			return false
		}
	}
	return true
}

func matchAttr(r parquettrace.SpanRow, c traceql.Clause) bool {
	if c.IsNum {
		if v, ok := r.AttrsInt[c.Field]; ok {
			return cmpNum(float64(v), c.Op, c.Num)
		}
		if v, ok := r.AttrsDouble[c.Field]; ok {
			return cmpNum(v, c.Op, c.Num)
		}
		return false
	}
	if v, ok := r.AttrsStr[c.Field]; ok {
		if c.Op == "!=" {
			return v != c.Str
		}
		return v == c.Str
	}
	return false
}

func cmpNum(a float64, op string, b float64) bool {
	switch op {
	case "=":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	}
	return false
}
