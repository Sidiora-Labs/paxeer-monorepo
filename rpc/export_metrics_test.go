package evmrpc

import "go.opentelemetry.io/otel/metric"

func SwapRequestLatencyForTest(h metric.Float64Histogram) (restore func()) {
	previous := metrics.requestLatencySeconds
	metrics.requestLatencySeconds = h
	return func() {
		metrics.requestLatencySeconds = previous
	}
}
