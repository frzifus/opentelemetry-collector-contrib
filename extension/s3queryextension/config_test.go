// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg)

	c, ok := cfg.(*Config)
	require.True(t, ok, "config should be *Config")

	assert.Equal(t, "localhost:3200", c.ServerConfig.NetAddr.Endpoint)
	assert.True(t, c.Traces.Enabled)
	assert.Equal(t, "traces/", c.Traces.Prefix)
}
