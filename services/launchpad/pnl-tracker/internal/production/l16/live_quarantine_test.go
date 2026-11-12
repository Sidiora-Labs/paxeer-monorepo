package l16

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
)

func parityWorker(t *testing.T) (*LiveTransferWorker, *internaltest.QueryCancellation) {
	t.Helper()
	ctx := context.Background()
	s, trace := inventoryCancelableService(t)
	inventoryCommittedHead(t, ctx, s.authority.pool)
	origin := repair63RecordedRPC(t)
	s.second, _ = inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == "eth_call" }, func(out map[string]any) { out["result"] = "0x" + strings.Repeat("0", 64) })
	identity, principal := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(s, l17.Repository{Pool: s.authority.pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.EnqueueOwned(ctx, principal); err != nil {
		t.Fatal(err)
	}
	return worker, trace
}

func TestLiveWalletQuarantinesParityBeforePortfolioCommit(t *testing.T) {
	ctx := context.Background()
	probe, trace := parityWorker(t)
	trace.Start(0)
	changed, err := probe.processWallet(ctx, realMarketCreator)
	queries := trace.Stop()
	if changed || !errors.Is(err, ErrParity) || len(queries) == 0 {
		t.Fatal(changed, queries, err)
	}
	var quarantines, snapshots int
	if err = probe.service.authority.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16_quarantine_v2),(SELECT count(*) FROM pnl.portfolio_snapshots)`).Scan(&quarantines, &snapshots); err != nil || quarantines != 1 || snapshots != 0 {
		t.Fatal(quarantines, snapshots, err)
	}
	if changed, err = probe.processWallet(ctx, realMarketCreator); changed || !errors.Is(err, ErrParity) {
		t.Fatal(changed, err)
	}
	candidate, cancellation := parityWorker(t)
	for i, query := range queries {
		if strings.EqualFold(query, "rollback") {
			continue
		}
		t.Run(fmt.Sprintf("cancel_%03d", i+1), func(t *testing.T) {
			cancellation.Start(i + 1)
			changed, err := candidate.processWallet(ctx, realMarketCreator)
			observed := cancellation.Stop()
			if changed || !errors.Is(err, context.Canceled) || len(observed) != i+1 || observed[i] != query {
				t.Fatal(changed, observed, query, err)
			}
			if err = candidate.service.authority.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.inventory_l16_quarantine_v2),(SELECT count(*) FROM pnl.portfolio_snapshots)`).Scan(&quarantines, &snapshots); err != nil || quarantines != 0 || snapshots != 0 {
				t.Fatal(quarantines, snapshots, err)
			}
		})
	}
}
