// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3queryextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension"

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/frontend"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/frontend/tempo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"
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
		_ = ln.Close()
		return err
	}

	go func() {
		if err := e.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.settings.Logger.Error("s3query server stopped", zap.Error(err))
		}
	}()

	return nil
}

func (e *s3QueryExtension) Shutdown(ctx context.Context) error {
	if e.server == nil {
		return nil
	}
	return e.server.Shutdown(ctx)
}

// buildFrontends constructs the enabled query frontends (Tempo/TraceQL, PromQL, LogQL).
func (e *s3QueryExtension) buildFrontends(ctx context.Context) ([]frontend.Frontend, error) {
	var fs []frontend.Frontend
	if e.cfg.Traces.Enabled {
		os, err := store.NewS3Store(ctx, e.cfg.S3)
		if err != nil {
			return nil, err
		}
		fs = append(fs, tempo.New(&store.Store{OS: os, Prefix: e.cfg.Traces.Prefix}))
	}
	// future: if e.cfg.Metrics.Enabled { fs = append(fs, promql.New(...)) }
	return fs, nil
}
