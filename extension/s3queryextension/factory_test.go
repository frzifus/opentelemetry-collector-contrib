// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension

import (
	"testing"

	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactoryCreatesExtension(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	if _, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg); err != nil {
		t.Fatal(err)
	}
}
