// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package frontend // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/frontend"

import "net/http"

// Frontend is one query-language / API surface (Tempo/TraceQL today; PromQL/LogQL later).
// The extension owns the HTTP server and the S3 store; each Frontend only mounts its routes.
type Frontend interface {
	Name() string
	Register(mux *http.ServeMux)
}
