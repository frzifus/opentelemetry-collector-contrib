// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tempo // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/frontend/tempo"

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/store"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/s3queryextension/internal/traceql"
)

// Frontend is the Tempo-compatible TraceQL query frontend. It implements
// frontend.Frontend (Name + Register) so the extension can mount it alongside
// future PromQL/LogQL frontends on the shared HTTP server.
type Frontend struct {
	store *store.Store
}

func New(s *store.Store) *Frontend { return &Frontend{store: s} }

func (*Frontend) Name() string { return "tempo" }

func (f *Frontend) Register(mux *http.ServeMux) {
	s := f.store
	mux.HandleFunc("GET /api/traces/{traceID}", func(w http.ResponseWriter, r *http.Request) {
		id, err := hex.DecodeString(r.PathValue("traceID"))
		if err != nil {
			http.Error(w, "bad trace id", http.StatusBadRequest)
			return
		}
		start, end := timeRange(r)
		td, found, err := s.TraceByID(r.Context(), id, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		b, err := (&ptrace.JSONMarshaler{}).MarshalTraces(td)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		q, err := traceql.ParseTraceQL(r.URL.Query().Get("q"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start, end := timeRange(r)
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}
		res, err := s.Search(r.Context(), q, start, end, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeSearch(w, res)
	})
}

func timeRange(r *http.Request) (int64, int64) {
	start := int64(0)
	end := int64(1)<<62 - 1
	if v := r.URL.Query().Get("start"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			start = n
		}
	}
	if v := r.URL.Query().Get("end"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			end = n
		}
	}
	return start, end
}

type searchTrace struct {
	TraceID           string `json:"traceID"`
	RootServiceName   string `json:"rootServiceName"`
	RootTraceName     string `json:"rootTraceName"`
	StartTimeUnixNano string `json:"startTimeUnixNano"`
	DurationMs        int64  `json:"durationMs"`
}

func writeSearch(w http.ResponseWriter, res []store.SearchResult) {
	out := struct {
		Traces []searchTrace `json:"traces"`
	}{}
	for _, r := range res {
		out.Traces = append(out.Traces, searchTrace{
			TraceID:           hex.EncodeToString(r.TraceID),
			RootServiceName:   r.RootServiceName,
			RootTraceName:     r.RootSpanName,
			StartTimeUnixNano: strconv.FormatInt(r.StartUnixNano, 10),
			DurationMs:        r.DurationNano / 1_000_000,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
