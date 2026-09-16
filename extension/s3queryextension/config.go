// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension"

import (
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"
)

// Config holds the extension's configuration.
type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`
	S3           store.S3Config          `mapstructure:"s3"`     // shared bucket/endpoint/creds
	Traces       SignalConfig            `mapstructure:"traces"` // POC frontend
	// Metrics SignalConfig `mapstructure:"metrics"` // future (spec §9)
	// Logs    SignalConfig `mapstructure:"logs"`    // future (spec §9)
}

// SignalConfig configures one signal frontend (traces/metrics/logs).
type SignalConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Prefix  string `mapstructure:"prefix"`
}
