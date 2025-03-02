// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func doNothingExportSink(_ context.Context, reqL []*prompb.WriteRequest) error {
	_ = reqL
	return nil
}

func TestWALCreation_nilConfig(t *testing.T) {
	config := (*WALConfig)(nil)
	pwal, err := newWAL(config, doNothingExportSink)
	require.Equal(t, err, errNilConfig)
	require.Nil(t, pwal)
}

func TestWALCreation_nonNilConfig(t *testing.T) {
	config := &WALConfig{Directory: t.TempDir()}
	pwal, err := newWAL(config, doNothingExportSink)
	require.NotNil(t, pwal)
	assert.Nil(t, err)
	assert.NoError(t, pwal.stop())
}

func orderByLabelValueForEach(reqL []*prompb.WriteRequest) {
	for _, req := range reqL {
		orderByLabelValue(req)
	}
}

func orderByLabelValue(wreq *prompb.WriteRequest) {
	// Sort the timeSeries by their labels.
	type byLabelMessage struct {
		label  *prompb.Label
		sample *prompb.Sample
	}

	for _, timeSeries := range wreq.Timeseries {
		bMsgs := make([]*byLabelMessage, 0, len(wreq.Timeseries)*10)
		for i := range timeSeries.Labels {
			bMsgs = append(bMsgs, &byLabelMessage{
				label:  &timeSeries.Labels[i],
				sample: &timeSeries.Samples[i],
			})
		}
		sort.Slice(bMsgs, func(i, j int) bool {
			return bMsgs[i].label.Value < bMsgs[j].label.Value
		})

		for i := range bMsgs {
			timeSeries.Labels[i] = *bMsgs[i].label
			timeSeries.Samples[i] = *bMsgs[i].sample
		}
	}

	// Now finally sort stably by timeseries value for
	// which just .String() is good enough for comparison.
	sort.Slice(wreq.Timeseries, func(i, j int) bool {
		ti, tj := wreq.Timeseries[i], wreq.Timeseries[j]
		return ti.String() < tj.String()
	})
}

func TestWALStopManyTimes(t *testing.T) {
	tempDir := t.TempDir()
	config := &WALConfig{
		Directory:         tempDir,
		TruncateFrequency: 60 * time.Microsecond,
		BufferSize:        1,
	}
	pwal, err := newWAL(config, doNothingExportSink)
	require.Nil(t, err)
	require.NotNil(t, pwal)

	// Ensure that invoking .stop() multiple times doesn't cause a panic, but actually
	// First close should NOT return an error.
	err = pwal.stop()
	require.Nil(t, err)
	for i := 0; i < 4; i++ {
		// Every invocation to .stop() should return an errAlreadyClosed.
		err = pwal.stop()
		require.Equal(t, err, errAlreadyClosed)
	}
}

func TestWAL_persist(t *testing.T) {
	// Unit tests that requests written to the WAL persist.
	config := &WALConfig{Directory: t.TempDir()}

	pwal, err := newWAL(config, doNothingExportSink)
	require.Nil(t, err)
	pwal.log = zap.Must(zap.NewDevelopment())

	// 1. Write out all the entries.
	reqL := []*prompb.WriteRequest{
		{
			Timeseries: []prompb.TimeSeries{
				{
					Labels:  []prompb.Label{{Name: "ts1l1", Value: "ts1k1"}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
				},
			},
		},
		{
			Timeseries: []prompb.TimeSeries{
				{
					Labels:  []prompb.Label{{Name: "ts2l1", Value: "ts2k1"}},
					Samples: []prompb.Sample{{Value: 2, Timestamp: 200}},
				},
				{
					Labels:  []prompb.Label{{Name: "ts1l1", Value: "ts1k1"}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
				},
			},
		},
	}

	ctx := context.Background()
	err = pwal.retrieveWALIndices()
	require.Nil(t, err)
	t.Cleanup(func() {
		assert.NoError(t, pwal.stop())
	})

	err = pwal.persistToWAL(reqL)
	require.Nil(t, err)

	// 2. Read all the entries from the WAL itself, guided by the indices available,
	// and ensure that they are exactly in order as we'd expect them.
	wal := pwal.wal
	start, err := wal.FirstIndex()
	require.Nil(t, err)
	end, err := wal.LastIndex()
	require.Nil(t, err)

	var reqLFromWAL []*prompb.WriteRequest
	for i := start; i <= end; i++ {
		req, err := pwal.readPrompbFromWAL(ctx, i)
		require.Nil(t, err)
		reqLFromWAL = append(reqLFromWAL, req)
	}

	orderByLabelValueForEach(reqL)
	orderByLabelValueForEach(reqLFromWAL)
	require.Equal(t, reqLFromWAL[0], reqL[0])
	require.Equal(t, reqLFromWAL[1], reqL[1])
}

func TestWAL_E2E(t *testing.T) {
	in := []*prompb.WriteRequest{
		series("mem_used_percent", 0, 0),
		series("mem_used_percent", 15, 34),
		series("mem_used_percent", 30, 99),
	}
	out := make([]*prompb.WriteRequest, 0, len(in))

	done := make(chan struct{})
	sink := func(ctx context.Context, reqs []*prompb.WriteRequest) error {
		out = append(out, reqs...)
		if len(out) >= len(in) {
			close(done)
		}
		return nil
	}

	wal, err := newWAL(&WALConfig{
		Directory: t.TempDir(),
	}, sink)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	ctx = contextWithLogger(ctx, log)

	if err := wal.run(ctx); err != nil {
		panic(err)
	}

	if err := wal.persistToWAL(in); err != nil {
		panic(err)
	}

	// wait until the tail routine is no longer busy
	wal.rNotify <- struct{}{}
	cancel()

	// wait until we received all series
	<-done
	require.Equal(t, in, out)
}

func series(name string, ts int64, value float64) *prompb.WriteRequest {
	return &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels:  []prompb.Label{{Name: "__name__", Value: name}},
				Samples: []prompb.Sample{{Value: value, Timestamp: ts}},
			},
		},
	}
}
