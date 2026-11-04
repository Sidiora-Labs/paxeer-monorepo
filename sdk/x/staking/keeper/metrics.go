package keeper

import (
	"context"
	"math"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("paxcosmos_x_staking_keeper")

	stakingMetrics = struct {
		amountOverflowTotal metric.Int64Counter
		delegateTotal       metric.Int64Counter
		delegateAmount      metric.Int64Gauge
		redelegateTotal     metric.Int64Counter
		redelegateAmount    metric.Int64Gauge
		undelegateTotal     metric.Int64Counter
		undelegateAmount    metric.Int64Gauge
	}{
		amountOverflowTotal: must(meter.Int64Counter(
			"staking_keeper_amount_overflow",
			metric.WithDescription("Successful staking amounts above MaxInt64, labeled by operation; amount gauges saturate at MaxInt64"),
			metric.WithUnit("{count}"),
		)),
		delegateTotal: must(meter.Int64Counter(
			"staking_keeper_delegate",
			metric.WithDescription("Number of delegation transactions"),
			metric.WithUnit("{count}"),
		)),
		delegateAmount: must(meter.Int64Gauge(
			"staking_keeper_last_delegate_amount",
			metric.WithDescription("Amount delegated in the last delegation transaction, saturated at MaxInt64; see staking_keeper_amount_overflow"),
			metric.WithUnit("{uhpx}"),
		)),
		redelegateTotal: must(meter.Int64Counter(
			"staking_keeper_redelegate",
			metric.WithDescription("Number of redelegation transactions"),
			metric.WithUnit("{count}"),
		)),
		redelegateAmount: must(meter.Int64Gauge(
			"staking_keeper_last_redelegate_amount",
			metric.WithDescription("Amount redelegated in the last redelegation transaction, saturated at MaxInt64; see staking_keeper_amount_overflow"),
			metric.WithUnit("{uhpx}"),
		)),
		undelegateTotal: must(meter.Int64Counter(
			"staking_keeper_undelegate",
			metric.WithDescription("Number of undelegation transactions"),
			metric.WithUnit("{count}"),
		)),
		undelegateAmount: must(meter.Int64Gauge(
			"staking_keeper_last_undelegate_amount",
			metric.WithDescription("Amount undelegated in the last undelegation transaction, saturated at MaxInt64; see staking_keeper_amount_overflow"),
			metric.WithUnit("{uhpx}"),
		)),
	}
)

func must[V any](v V, err error) V {
	if err != nil {
		panic(err)
	}
	return v
}

func recordStakingAmount(ctx context.Context, amount sdk.Int, gauge metric.Int64Gauge, operation string) {
	value := int64(math.MaxInt64)
	if amount.IsInt64() {
		value = amount.Int64()
	} else {
		stakingMetrics.amountOverflowTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", operation)))
	}
	gauge.Record(ctx, value)
}
