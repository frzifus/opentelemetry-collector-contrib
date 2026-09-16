// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/metadata"
)

// NewFactory creates a factory for the s3query extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(metadata.Type, createDefaultConfig, createExtension, metadata.ExtensionStability)
}

func createDefaultConfig() component.Config {
	sc := confighttp.NewDefaultServerConfig()
	sc.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:3200",
	}
	return &Config{
		ServerConfig: sc,
		Traces:       SignalConfig{Enabled: true, Prefix: "traces/"},
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newS3QueryExtension(cfg.(*Config), set), nil
}
