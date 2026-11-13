package store

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
)

func TestLaunchedChainRevenuePersistsAndRetires(t *testing.T) {
	ctx := context.Background()
	_, pool := internaltest.NewPostgres(t)
	store := New(pool)
	chain := LaunchedChain{
		Chain: 31, ChainID: "125", Pool: "0x0000000000000000000000000000000000000031",
		Token: "0x0000000000000000000000000000000000000032", FeeRight: "right-31",
		Creator: "0x0000000000000000000000000000000000000033", BlockNumber: 101,
		TxHash: "0x0000000000000000000000000000000000000000000000000000000000000031",
	}
	if err := store.InsertLaunchedChain(ctx, chain); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLaunchedChain(ctx, chain); err != nil {
		t.Fatalf("same chain identity was not idempotent: %v", err)
	}
	changed := chain
	changed.Token = "0x0000000000000000000000000000000000000099"
	if err := store.InsertLaunchedChain(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed chain identity: %v", err)
	}
	epoch := ChainRevenueEpoch{Chain: 31, Epoch: "2", Amount: "100", SnapshotBlock: "102", EligibleSupply: "80", OpenedBlock: 103}
	if err := store.InsertChainRevenueEpoch(ctx, epoch); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertChainRevenueEpoch(ctx, epoch); err != nil {
		t.Fatalf("same epoch was not idempotent: %v", err)
	}
	changedEpoch := epoch
	changedEpoch.Amount = "101"
	if err := store.InsertChainRevenueEpoch(ctx, changedEpoch); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed epoch amount: %v", err)
	}
	claim := ChainRevenueClaim{Chain: 31, Epoch: "2", Holder: "0x0000000000000000000000000000000000000034", Amount: "10", BlockNumber: 104, TxHash: "0x0000000000000000000000000000000000000000000000000000000000000034"}
	if err := store.InsertChainRevenueClaim(ctx, claim); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertChainRevenueClaim(ctx, claim); err != nil {
		t.Fatalf("same claim was not idempotent: %v", err)
	}
	changedClaim := claim
	changedClaim.Amount = "11"
	if err := store.InsertChainRevenueClaim(ctx, changedClaim); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed claim amount: %v", err)
	}
	if err := store.RetireLaunchedChain(ctx, 31, 105); err != nil {
		t.Fatal(err)
	}
	if err := store.RetireLaunchedChain(ctx, 31, 105); err != nil {
		t.Fatalf("same retirement was not idempotent: %v", err)
	}
	if err := store.RetireLaunchedChain(ctx, 31, 106); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed retirement height: %v", err)
	}
	var status string
	var retired int64
	if err := pool.QueryRow(ctx, `SELECT status, retired_block FROM indexer.launched_chains WHERE chain=31`).Scan(&status, &retired); err != nil || status != "retired" || retired != 105 {
		t.Fatalf("stored retirement = %q at %d: %v", status, retired, err)
	}
	pool.Close()
	for _, call := range []func() error{
		func() error { return store.InsertLaunchedChain(ctx, chain) },
		func() error { return store.InsertChainRevenueEpoch(ctx, epoch) },
		func() error { return store.InsertChainRevenueClaim(ctx, claim) },
		func() error { return store.RetireLaunchedChain(ctx, 31, 105) },
	} {
		if err := call(); err == nil {
			t.Fatal("unavailable chain persistence admitted")
		}
	}

}
