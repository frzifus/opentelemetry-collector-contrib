// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension"

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/frontend"
)

type s3QueryExtension struct {
	cfg      *Config
	settings extension.Settings
	server   *http.Server
}

func newS3QueryExtension(cfg *Config, settings extension.Settings) *s3QueryExtension {
	return &s3QueryExtension{
		cfg:      cfg,
		settings: settings,
	}
}

func (e *s3QueryExtension) Start(ctx context.Context, host component.Host) error {
	fs, err := e.buildFrontends(ctx)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	for _, f := range fs {
		f.Register(mux)
		e.settings.Logger.Info("registered s3query frontend", zap.String("frontend", f.Name()))
	}

	ln, err := e.cfg.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}

	e.server, err = e.cfg.ServerConfig.ToServer(ctx, host.GetExtensions(), e.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}

	go func() {
		_ = e.server.Serve(ln)
	}()

	return nil
}

func (e *s3QueryExtension) Shutdown(ctx context.Context) error {
	if e.server == nil {
		return nil
	}
	return e.server.Close()
}

// buildFrontends constructs the enabled query frontends (Tempo/TraceQL, PromQL, LogQL).
// For Task 13: returns empty slice — the Tempo frontend is wired in Task 14.
func (e *s3QueryExtension) buildFrontends(ctx context.Context) ([]frontend.Frontend, error) {
	// Trace frontend (Tempo/TraceQL) is wired in Task 14 once the tempo package exists.
	return nil, nil
}
