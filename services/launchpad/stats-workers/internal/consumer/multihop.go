package consumer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/jackc/pgx/v5"
)

// MultihopEvent is a decoded native Router MultihopSwap event (emitted by
// Router.swapTokenForToken), fanned out by the indexer. It records cross-token
// trades routed token in -> intermediate USDL -> token out, feeding the
// cross-token-swap history endpoints.
//
// This is the NATIVE DEX multihop and is distinct from the MetaAG aggregator's
// MultiHopSwap event (a separate, out-of-scope contract). The on-chain event
// carries sender/tokenIn/tokenOut/amountIn/intermediateUsdl/amountOut; the
// indexer enriches poolIn/poolOut/feeIn/feeOut (all NOT NULL) before fan-out.
type MultihopEvent struct {
	Sender           string
	TokenIn          string
	TokenOut         string
	PoolIn           string
	PoolOut          string
	AmountIn         string
	IntermediateUsdl string
	AmountOut        string
	FeeIn            string
	FeeOut           string
	BlockTimestamp   int64
	TxHash           string
	LogIndex         int
}

// MultihopConsumer records cross-token swaps. Ports MultihopConsumer.
type MultihopConsumer struct {
	store  *store.Store
	logger *slog.Logger
}

// NewMultihopConsumer builds a MultihopConsumer.
func NewMultihopConsumer(st *store.Store, logger *slog.Logger) *MultihopConsumer {
	return &MultihopConsumer{store: st, logger: logger}
}

// ProcessEvent records a cross-token swap, idempotently. Ports
// MultihopConsumer.processEvent.
func (c *MultihopConsumer) ProcessEvent(ctx context.Context, ev MultihopEvent) error {
	if err := c.store.InsertCrossTokenSwap(ctx, multihopRow(ev)); err != nil {
		return err
	}
	c.logger.Info("cross-token swap recorded in stats",
		slog.String("txHash", ev.TxHash), slog.String("sender", ev.Sender),
		slog.String("tokenIn", ev.TokenIn), slog.String("tokenOut", ev.TokenOut))
	return nil
}

func multihopRow(ev MultihopEvent) store.CrossTokenSwapRow {
	return store.CrossTokenSwapRow{
		ID:               fmt.Sprintf("%s-%d", ev.TxHash, ev.LogIndex),
		Sender:           ev.Sender,
		TokenIn:          ev.TokenIn,
		TokenOut:         ev.TokenOut,
		PoolIn:           ev.PoolIn,
		PoolOut:          ev.PoolOut,
		AmountIn:         ev.AmountIn,
		IntermediateUsdl: ev.IntermediateUsdl,
		AmountOut:        ev.AmountOut,
		FeeIn:            ev.FeeIn,
		FeeOut:           ev.FeeOut,
		BlockTimestamp:   ev.BlockTimestamp,
		TxHash:           ev.TxHash,
	}
}

// ProjectCanonicalTx records a source-bound route. Its txHash-logIndex row is
// also the durable replay checkpoint; no checkpoint can precede the history.
func (c *MultihopConsumer) ProjectCanonicalTx(ctx context.Context, tx pgx.Tx, ev MultihopEvent) (bool, error) {
	if c == nil || c.store == nil || tx == nil {
		return false, ErrLegacyBridgeIntegrity
	}
	want := multihopRow(ev)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "stats:multihop:"+want.ID); err != nil {
		return false, err
	}
	var existed bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM stats.cross_token_swaps WHERE id=$1)", want.ID).Scan(&existed); err != nil {
		return false, err
	}
	if err := c.store.WithTx(tx).InsertCrossTokenSwap(ctx, want); err != nil {
		return false, err
	}
	var got store.CrossTokenSwapRow
	err := tx.QueryRow(ctx, `SELECT id,sender,token_in,token_out,pool_in,pool_out,amount_in,intermediate_usdl,amount_out,fee_in,fee_out,block_timestamp,tx_hash FROM stats.cross_token_swaps WHERE id=$1`, want.ID).Scan(&got.ID, &got.Sender, &got.TokenIn, &got.TokenOut, &got.PoolIn, &got.PoolOut, &got.AmountIn, &got.IntermediateUsdl, &got.AmountOut, &got.FeeIn, &got.FeeOut, &got.BlockTimestamp, &got.TxHash)
	if err != nil {
		return false, err
	}
	if got != want {
		return false, ErrLegacyBridgeIntegrity
	}
	return !existed, nil
}
