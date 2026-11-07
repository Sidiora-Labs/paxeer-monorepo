package sequencer

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	receiptLatency metric.Float64Histogram
	walSync        metric.Float64Histogram
	standbyAck     metric.Float64Histogram
	openSeqGauge   metric.Int64ObservableGauge
	sealedGauge    metric.Int64ObservableGauge
	openSeq        atomic.Uint64
	sealedHeight   atomic.Int64
}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	m := new(Metrics)
	var err error
	m.receiptLatency, err = meter.Float64Histogram(
		"sequencer_receipt_latency_seconds",
		metric.WithDescription("Duration from transaction acceptance to receipt"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	m.walSync, err = meter.Float64Histogram(
		"sequencer_wal_sync_seconds",
		metric.WithDescription("Duration of sequencer WAL sync"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	m.standbyAck, err = meter.Float64Histogram(
		"sequencer_standby_ack_seconds",
		metric.WithDescription("Duration of sequencer standby acknowledgement"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	m.openSeqGauge, err = meter.Int64ObservableGauge(
		"sequencer_open_seq",
		metric.WithDescription("Current open sequencer sequence number"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(int64(m.openSeq.Load()))
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	m.sealedGauge, err = meter.Int64ObservableGauge(
		"sequencer_sealed_height",
		metric.WithDescription("Last sealed sequencer block height"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(m.sealedHeight.Load())
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Metrics) ObserveReceipt(d time.Duration) {
	m.receiptLatency.Record(context.Background(), d.Seconds())
}

func (m *Metrics) ObserveWALSync(d time.Duration) {
	m.walSync.Record(context.Background(), d.Seconds())
}

func (m *Metrics) ObserveStandbyAck(d time.Duration) {
	m.standbyAck.Record(context.Background(), d.Seconds())
}

func (m *Metrics) SetOpenSeq(seq uint64) {
	m.openSeq.Store(seq)
}

func (m *Metrics) SetSealedHeight(h int64) {
	m.sealedHeight.Store(h)
}
