// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceql // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Clause struct {
	Field string
	Op    string
	Str   string
	Num   float64
	IsNum bool
}

type Query struct {
	ServiceName     string
	SpanName        string
	MinDurationNano int64
	StatusCode      *int32
	Attrs           []Clause
}

func ParseTraceQL(s string) (Query, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	var q Query
	if strings.TrimSpace(s) == "" {
		return q, nil
	}
	for _, part := range strings.Split(s, "&&") {
		c, err := parseClause(strings.TrimSpace(part))
		if err != nil {
			return q, err
		}
		if err := applyClause(&q, c); err != nil {
			return q, err
		}
	}
	return q, nil
}

var ops = []string{">=", "<=", "!=", "=", ">", "<"}

func parseClause(p string) (Clause, error) {
	for _, op := range ops {
		if i := strings.Index(p, op); i >= 0 {
			field := strings.TrimSpace(p[:i])
			val := strings.TrimSpace(p[i+len(op):])
			c := Clause{Field: strings.TrimPrefix(field, "."), Op: op}
			if strings.HasPrefix(val, `"`) {
				c.Str = strings.Trim(val, `"`)
			} else if d, err := time.ParseDuration(val); err == nil && (c.Field == "duration") {
				c.Num = float64(d.Nanoseconds())
				c.IsNum = true
			} else if n, err := strconv.ParseFloat(val, 64); err == nil {
				c.Num = n
				c.IsNum = true
			} else {
				c.Str = val // keyword (status) or bare word
			}
			return c, nil
		}
	}
	return Clause{}, fmt.Errorf("no operator in clause %q", p)
}

func applyClause(q *Query, c Clause) error {
	switch c.Field {
	case "service.name":
		q.ServiceName = c.Str
	case "name":
		q.SpanName = c.Str
	case "duration":
		if c.Op == ">" || c.Op == ">=" {
			q.MinDurationNano = int64(c.Num)
		} else {
			return fmt.Errorf("unsupported operator %q for duration", c.Op)
		}
	case "status":
		if c.Op != "=" {
			return fmt.Errorf("unsupported operator %q for status", c.Op)
		}
		code := int32(0)
		switch c.Str {
		case "error":
			code = 2
		case "ok":
			code = 1
		case "unset":
			code = 0
		}
		q.StatusCode = &code
	default:
		q.Attrs = append(q.Attrs, c)
	}
	return nil
}
