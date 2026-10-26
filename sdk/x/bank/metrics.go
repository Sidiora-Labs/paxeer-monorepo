package bank

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("paxcosmos_x_bank")

	finerGrainedBuckets = metric.WithExplicitBucketBoundaries(
		0.000025, 0.000050, 0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.010, 0.020, 0.050, 0.075, 0.1, 0.25, 0.5, 1, 10,
	)

	bankMetrics = struct {
		initGenesisDuration metric.Float64Histogram
	}{
		initGenesisDuration: initGenesisDuration(),
	}
)

func initGenesisDuration() metric.Float64Histogram {
	duration, err := meter.Float64Histogram(
		"init_genesis_duration",
		metric.WithDescription("Duration of bank genesis decoding in seconds"),
		finerGrainedBuckets,
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(err)
	}
	return duration
}
