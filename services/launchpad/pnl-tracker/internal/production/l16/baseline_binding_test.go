package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestRecordedBaselinePreparationAndAttestationIntegrity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	a := service.authority
	wallet := "0x" + strings.Repeat("e", 40)
	partition := "125:" + wallet + ":" + realMarketToken
	in, err := a.PreparePartition(ctx, partition)
	if err != nil || in.Lease.Key.PartitionKey != partition || in.NextSequence != "1" || len(in.CheckpointDigest) != 64 {
		t.Fatal(in, err)
	}
	for _, p := range []string{"", "126:" + wallet + ":" + realMarketToken, "125:invalid:invalid"} {
		if _, err = a.PreparePartition(ctx, p); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(p, err)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if err = a.VerifyBootstrapTx(ctx, tx, in); err != nil {
		t.Fatal(err)
	}
	changed := in
	changed.CheckpointDigest = accounting.Digest(strings.Repeat("0", 64))
	if err = a.VerifyBootstrapTx(ctx, tx, changed); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	if err = a.VerifyBootstrapTx(ctx, nil, in); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	att, digest, err := a.readBaseline(ctx, tx, partition)
	if err != nil || digest != string(in.CheckpointDigest) || att.Wallet != wallet || att.Token != realMarketToken {
		t.Fatal(att, digest, err)
	}
	named, owner, epoch, err := a.readNamedBaselines(ctx, tx, partition)
	if err != nil || len(named) != 1 || named[0].att != att || owner != att.SourceOwner || epoch != att.SourceEpoch {
		t.Fatal(named, owner, epoch, err)
	}
	if _, _, err = a.readBaseline(ctx, tx, "125:"+realMarketCreator+":"+realMarketToken); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if _, _, _, err = a.readNamedBaselines(ctx, tx, "125:"+realMarketCreator+":"+realMarketToken); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{[]byte(`{`), []byte(`{}`)} {
		t.Run(string(raw), func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(tx)
			if _, err = tx.Exec(ctx, "UPDATE pnl.inventory_l16_baselines SET attestation=$1 WHERE partition=$2", raw, partition); err != nil {
				t.Fatal(err)
			}
			if _, _, err = a.readBaseline(ctx, tx, partition); !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatal(err)
			}
			if _, _, _, err = a.readNamedBaselines(ctx, tx, partition); !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatal(err)
			}
		})
	}
	if err = a.VerifyFreshSameOwnerTx(ctx, nil, "g1", accounting.FillV1{}, accounting.PartitionPosition{}); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
	if err = a.persistFreshUseTx(ctx, nil, qualifiedBaselineUse{}); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatal(err)
	}
}

func TestBaselineAheadEnrolledWalletAndRefusalBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	a := service.authority
	archive := *a
	archive.rpc = service.second
	ahead, err := NewBaselineAhead(a, &archive, 32)
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []int{0, 257} {
		if _, err = NewBaselineAhead(a, &archive, batch); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if _, err = NewBaselineAhead(a, a, 1); err == nil {
		t.Fatal("single authority accepted")
	}
	divergent := archive
	divergent.generation = "other"
	if _, err = NewBaselineAhead(a, &divergent, 1); err == nil {
		t.Fatal("different generation accepted")
	}
	identity, p := inventoryIdentityFixture(t, "0x"+strings.Repeat("e", 40))
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.EnqueueOwned(ctx, p); err != nil {
		t.Fatal(err)
	}
	candidates, err := ahead.candidates(ctx)
	if err != nil || len(candidates) != 1 || candidates[0].wallet != p.Wallet {
		t.Fatal(candidates, err)
	}
	if issued, err := ahead.RunOnce(ctx); err != nil || issued != 1 {
		t.Fatal(issued, err)
	}
	if issued, err := ahead.RunOnce(ctx); err != nil || issued != 0 {
		t.Fatal(issued, err)
	}
	if err = ahead.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= baselineAheadRefusalCap; i++ {
		ahead.refuse(fmt.Sprint(i))
	}
	ahead.refuse(fmt.Sprint(baselineAheadRefusalCap))
	if len(ahead.refused) != baselineAheadRefusalCap || len(ahead.order) != baselineAheadRefusalCap || ahead.order[0] != "1" {
		t.Fatal(len(ahead.refused), len(ahead.order), ahead.order[0])
	}
	if _, exists := ahead.refused["0"]; exists {
		t.Fatal("oldest refusal retained")
	}
	canceled, done := context.WithCancel(ctx)
	done()
	if _, err = ahead.RunOnce(canceled); err == nil {
		t.Fatal("cancelled database read accepted")
	}
}

func TestSameOwnerMovementAttribution(t *testing.T) {
	buy, p := inventoryWireFixture(t, 1, "buy")
	sell, _ := inventoryWireFixture(t, 2, "sell")
	wallet, token, pool := *p.Enrichment.Attribution.Payer, buy.Token.Address, buy.PoolAddress
	moves := []movement{}
	for i, f := range []*accounting.FillV1{&buy, &sell} {
		kind, from, to := "output-transfer", pool, wallet
		if f.Side == sideSell {
			kind, from, to = "input-transfer", wallet, pool
		}
		log := fmt.Sprint(i + 1)
		f.Attribution = accounting.Attribution{TransactionSender: wallet, Payer: &wallet, Recipient: &wallet, Trader: &wallet, Quality: "exact_receipt", Evidence: []accounting.EvidenceRef{{Kind: kind, ArtifactID: f.Position.TransactionHash, ArtifactSHA256: f.Provenance.ReceiptSHA256, LogIndex: &log}}}
		moves = append(moves, movement{TransactionHash: f.Position.TransactionHash, LogIndex: uint32(i + 1), Height: 100, BlockHash: f.Position.BlockHash, From: from, To: to, Amount: f.TokenAmountRaw})
	}
	fills := []accounting.FillV1{buy, sell}
	balance, err := replaySameOwnerMovements(wallet, token, pool, moves, fills)
	if err != nil || balance.String() != "5" {
		t.Fatal(balance, err)
	}
	tests := []struct {
		name   string
		change func(*[]movement, *[]accounting.FillV1)
	}{
		{"duplicate", func(m *[]movement, _ *[]accounting.FillV1) { *m = append(*m, (*m)[0]) }},
		{"missing receipt", func(_ *[]movement, f *[]accounting.FillV1) { (*f)[0].Attribution.Evidence = nil }},
		{"unavailable", func(_ *[]movement, f *[]accounting.FillV1) { (*f)[0].Attribution.Quality = "unavailable" }},
		{"negative balance", func(m *[]movement, _ *[]accounting.FillV1) { (*m)[0], (*m)[1] = (*m)[1], (*m)[0] }},
		{"unmatched fill", func(m *[]movement, _ *[]accounting.FillV1) { *m = (*m)[:1] }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var f []accounting.FillV1
			raw, e := json.Marshal(fills)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(raw, &f); e != nil {
				t.Fatal(e)
			}
			m := append([]movement(nil), moves...)
			tc.change(&m, &f)
			if _, e = replaySameOwnerMovements(wallet, token, pool, m, f); e == nil {
				t.Fatal("inconsistent movement attribution accepted")
			}
		})
	}
	unrelated := moves[0]
	unrelated.From = pool
	unrelated.To = pool
	if balance, err = replaySameOwnerMovements(wallet, token, pool, []movement{unrelated}, nil); err != nil || balance.Sign() != 0 {
		t.Fatal(balance, err)
	}
}
