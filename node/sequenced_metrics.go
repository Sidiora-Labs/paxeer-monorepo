package app

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	sequencedRejectNoBatch     = "no_batch"
	sequencedRejectBatchDecode = "batch_decode"
	sequencedRejectBatchTxs    = "batch_txs"
	sequencedRejectOrder       = "order"
	sequencedRejectProposer    = "proposer"
)

type sequencedMetricCounters struct {
	proposalRejected metric.Int64Counter
	blockHalt        metric.Int64Counter
}

var sequencedMetrics = newSequencedMetrics(meter)

func newSequencedMetrics(m metric.Meter) sequencedMetricCounters {
	return sequencedMetricCounters{
		proposalRejected: must(m.Int64Counter(
			"app_sequenced_proposal_rejected",
			metric.WithDescription("Sequenced proposals rejected by reason"),
			metric.WithUnit("{count}"),
		)),
		blockHalt: must(m.Int64Counter(
			"app_sequenced_block_halt",
			metric.WithDescription("Sequenced block halts"),
			metric.WithUnit("{count}"),
		)),
	}
}

func recordSequencedReject(ctx context.Context, reason string) {
	sequencedMetrics.proposalRejected.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

func recordSequencedHalt(ctx context.Context) {
	sequencedMetrics.blockHalt.Add(ctx, 1)
}
