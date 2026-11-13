package l16

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestLiveWalletPublishesEmptyCompletePortfolio(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	wallet := "0x" + strings.Repeat("e", 40)
	identity, principal := inventoryIdentityFixture(t, wallet)
	worker, err := NewLiveTransferWorker(s, l17.Repository{Pool: pool}, identity, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.EnqueueOwned(ctx, principal); err != nil {
		t.Fatal(err)
	}
	if count, err := worker.RunOnce(ctx); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	summary, err := worker.RebuildOwned(ctx, principal)
	if err != nil || summary.PositionCount != 0 || summary.BasisQuality != "complete" {
		t.Fatal(summary, err)
	}
	if changed, err := worker.processWallet(ctx, wallet); err != nil || !changed {
		t.Fatal(changed, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pnl.portfolio_history_snapshots WHERE id=$1`, summary.SnapshotID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err = worker.CancelOwned(ctx, auth.IdentityPrincipal{}); err == nil {
		t.Fatal("invalid cancellation accepted")
	}
	if err = worker.CancelOwned(ctx, principal); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET account_id='33333333-3333-4333-8333-333333333333' WHERE wallet=$1`, wallet); err != nil {
		t.Fatal(err)
	}
	if err = worker.CancelOwned(ctx, principal); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET account_id=$1 WHERE wallet=$2`, principal.AccountID, wallet); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		height int64
		hash   *string
	}{{height: 9223372036854775807}, {height: func() int64 {
		var value int64
		if err := pool.QueryRow(ctx, `SELECT accepted_height FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, wallet).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}(), hash: str("0x" + strings.Repeat("0", 64))}} {
		if err = worker.EnqueueOwned(ctx, principal); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET accepted_height=$1,accepted_hash=$2 WHERE wallet=$3`, tc.height, tc.hash, wallet); err != nil {
			t.Fatal(err)
		}
		if _, err = worker.processWallet(ctx, wallet); !errors.Is(err, delivery.ErrIntegrity) {
			t.Fatal(err)
		}
	}
}

func TestInventoryRuntimeRequiresQualifiedAuthorities(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	for _, change := range []func(*Service){func(s *Service) { s.authority = nil }, func(s *Service) { s.second = s.authority.rpc }, func(s *Service) { s.nodeIDs[1] = s.nodeIDs[0] }, func(s *Service) { s.release = strings.Repeat("g", 64) }, func(s *Service) { s.outbox = nil }} {
		copy := *s
		change(&copy)
		if _, err := NewService(copy.authority, copy.second, copy.nodeIDs, copy.release, copy.outbox); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(err)
		}
	}
	if digestHex(strings.Repeat("g", 64)) || partitionHash(strings.Repeat("x", 64)) || partitionHash("short") {
		t.Fatal("invalid digest accepted")
	}
	if _, err := NewAccountingConsumer(pool, "bad schema", "pnl", "g1", "owner", "v1", s.authority); err == nil {
		t.Fatal("invalid schema accepted")
	}
	if _, err := NewAccountingConsumerWithEpochGuard(pool, "pnl", "pnl", "g1", "owner", "v1", s.authority, nil); err == nil {
		t.Fatal("missing epoch guard accepted")
	}
	if _, err := NewAccountingConsumerWithEpochGuard(pool, "bad schema", "pnl", "g1", "owner", "v1", s.authority, s.authority.epochGuard); err == nil {
		t.Fatal("invalid guarded schema accepted")
	}
	if _, err := NewAccountingConsumerWithRuntime(pool, "pnl", "pnl", "g1", "owner", "v1", s.authority, s.authority.epochGuard, delivery.StreamWriter{}); err == nil {
		t.Fatal("unqualified stream accepted")
	}
	if _, err := NewAccountingConsumerWithRuntime(pool, "bad schema", "pnl", "g1", "owner", "v1", s.authority, s.authority.epochGuard, delivery.StreamWriter{Schema: "api_stream"}); err == nil {
		t.Fatal("invalid runtime schema accepted")
	}
	empty := &AccountingConsumer{}
	if err := empty.Bootstrap(ctx, delivery.BootstrapInput{}); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if err := empty.Consume(ctx, "invalid", nil, nil); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if err := empty.PrepareAndConsume(ctx, "invalid", nil, nil); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if err := empty.PreparePartition(ctx, "invalid"); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	c, err := NewAccountingConsumer(pool, "pnl", "pnl", "g1", "owner", "v1", s.authority)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Consume(ctx, "invalid", nil, nil); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	if err = c.Bootstrap(ctx, delivery.BootstrapInput{}); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	for _, request := range []struct {
		wallet, token string
		height        int64
	}{{"invalid", realMarketToken, 1}, {realMarketCreator, realMarketToken, 1}} {
		if _, err = s.liveRebuildTx(ctx, tx, request.wallet, request.token, request.height); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(err)
		}
	}
}
