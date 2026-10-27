package oteltest

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var (
	installOnce     sync.Once
	installedReader *Reader
)

type Reader struct {
	reader *sdkmetric.ManualReader
}

func Install() *Reader {
	installOnce.Do(func() {
		manual := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(manual))
		installedReader = &Reader{reader: manual}
		otel.SetMeterProvider(provider)
	})
	return installedReader
}

func (r *Reader) collect() metricdata.ResourceMetrics {
	var data metricdata.ResourceMetrics
	if err := r.reader.Collect(context.Background(), &data); err != nil {
		panic(fmt.Errorf("oteltest: collect metrics: %w", err))
	}
	return data
}

func (r *Reader) HistogramCount(meter, name string) uint64 {
	var count uint64
	for _, scope := range r.collect().ScopeMetrics {
		if scope.Scope.Name != meter {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			switch data := m.Data.(type) {
			case metricdata.Histogram[int64]:
				for _, point := range data.DataPoints {
					count += point.Count
				}
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					count += point.Count
				}
			}
		}
	}
	return count
}

func (r *Reader) CounterSum(meter, name string) int64 {
	var sum int64
	for _, scope := range r.collect().ScopeMetrics {
		if scope.Scope.Name != meter {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			if data, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, point := range data.DataPoints {
					sum += point.Value
				}
			}
		}
	}
	return sum
}

func (r *Reader) GaugeInt(meter, name string) (int64, bool) {
	var sum int64
	var found bool
	for _, scope := range r.collect().ScopeMetrics {
		if scope.Scope.Name != meter {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			if data, ok := m.Data.(metricdata.Gauge[int64]); ok {
				for _, point := range data.DataPoints {
					sum += point.Value
					found = true
				}
			}
		}
	}
	return sum, found
}

type LegacySink struct {
	sink *metrics.InmemSink
}

func InstallLegacySink(t testing.TB) *LegacySink {
	t.Helper()
	sink := metrics.NewInmemSink(time.Hour, time.Hour)
	config := metrics.DefaultConfig("oteltest")
	config.EnableHostname = false
	config.EnableRuntimeMetrics = false
	if _, err := metrics.NewGlobal(config, sink); err != nil {
		t.Fatalf("oteltest: install legacy sink: %v", err)
	}
	t.Cleanup(func() {
		if _, err := metrics.NewGlobal(config, &metrics.BlackholeSink{}); err != nil {
			t.Errorf("oteltest: install legacy blackhole sink: %v", err)
		}
	})
	return &LegacySink{sink: sink}
}

func (s *LegacySink) Keys() []string {
	names := make(map[string]struct{})
	for _, interval := range s.sink.Data() {
		interval.RLock()
		for _, counter := range interval.Counters {
			names[counter.Name] = struct{}{}
		}
		for _, gauge := range interval.Gauges {
			names[gauge.Name] = struct{}{}
		}
		for _, sample := range interval.Samples {
			names[sample.Name] = struct{}{}
		}
		interval.RUnlock()
	}
	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}
