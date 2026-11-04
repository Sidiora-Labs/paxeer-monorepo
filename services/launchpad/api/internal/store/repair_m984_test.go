package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

const repairM984Token = "0x3333333333333333333333333333333333333333"

// The fixture goes through the production accounting producer and strict wire
// codec before entering the same projection reconciliation used by SQL reads.
func repairM984Fill(t *testing.T, side, sender, owner string, height, timestamp int64, token, paid, usdl string) (accounting.FillV1, traderTokenTransfer) {
	t.Helper()
	position := accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(height, 10), TransactionIndex: "0", LogIndex: "2", TransactionHash: fmt.Sprintf("0x%064x", height), BlockHash: fmt.Sprintf("0x%064x", height+1000)}
	receipt := fmt.Sprintf("%064x", height+2000)
	logIndex := "1"
	kind, from, to, disposition := "output-transfer", realizedPool, owner, "distributed_usdl"
	if side == "sell" {
		kind, from, to, disposition = "input-transfer", owner, realizedPool, "retained_token_reserve"
	}
	f := accounting.FillV1{
		Version: accounting.FillVersion, Position: position, ActualEmitter: realizedPool, PoolAddress: realizedPool,
		Token: accounting.Asset{Address: repairM984Token, Decimals: 18}, Settlement: accounting.Asset{Address: "0x4444444444444444444444444444444444444444", Decimals: 6},
		Side: side, TokenAmountRaw: token, PaidInputRaw: paid, USDLAmountRaw: usdl, FeeRaw: "0", InputDerivationQuality: "receipt_incomplete", FeeDisposition: disposition,
		Attribution: accounting.Attribution{TransactionSender: sender, Trader: &sender, Payer: &owner, Recipient: &owner, Quality: "exact_receipt", Evidence: []accounting.EvidenceRef{{Kind: kind, ArtifactSHA256: receipt, ArtifactID: position.TransactionHash, LogIndex: &logIndex}}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  accounting.Provenance{DeploymentID: "repair-m984", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: receipt, HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: strconv.FormatInt(timestamp, 10),
	}
	f.SourceFillID = accounting.SourceFillIdentity(position, f.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(f)
	if err != nil {
		t.Fatal(err)
	}
	partition := accounting.PartitionPosition{ProjectionVersion: "accounting-v2", PartitionKind: "market", PartitionKey: "125:" + realizedPool, Sequence: "1", SourceEventID: f.SourceFillID, PayloadDigest: digest, SourceOrder: position}
	partition.EffectIdentity = accounting.EffectIdentity(f.SourceFillID, "market", "accounting-v2", "", "market", partition.PartitionKey)
	publication, err := accounting.NewMarketPublication(f, []accounting.PartitionPosition{partition})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := accounting.DecodePublicationV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	transferPosition := position
	transferPosition.LogIndex = logIndex
	movement := traderTokenTransfer{Token: repairM984Token, From: from, To: to, AmountRaw: token, Position: transferPosition, Provenance: f.Provenance}
	raw, err = json.Marshal(movement)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &movement); err != nil {
		t.Fatal(err)
	}
	return *decoded.Fill, movement
}

func repairM984Movement(height int64, from, to, amount string) traderTokenTransfer {
	return traderTokenTransfer{Token: repairM984Token, From: from, To: to, AmountRaw: amount,
		Position:   accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(height, 10), TransactionIndex: "0", LogIndex: "0", TransactionHash: fmt.Sprintf("0x%064x", height), BlockHash: fmt.Sprintf("0x%064x", height+1000)},
		Provenance: accounting.Provenance{ReceiptSHA256: fmt.Sprintf("%064x", height+2000)}}
}

func repairM984Snapshot(t *testing.T, fills []accounting.FillV1, movements []traderTokenTransfer, from, end int64) (*LeaderboardSnapshot, int) {
	t.Helper()
	rows, err := traderProjectionRows(fills, movements)
	if err != nil {
		t.Fatal(err)
	}
	entries, excluded, err := RealizedTraderPnL(rows, from, end)
	if err != nil {
		t.Fatal(err)
	}
	watermark := ChainWatermark{ChainID: "125", Height: "30", BlockHash: fmt.Sprintf("0x%064x", 1030), ProjectionRevision: "4", ObservedAt: "1970-01-01T00:33:20.000Z", Integrity: "healthy"}
	return traderLeaderboardSnapshot("trader", "24h", end, entries, excluded, watermark), excluded
}

func TestRepairM_984_TransfersCannotReuseStaleBasis(t *testing.T) {
	buy, buyMove := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 10, 900, "100", "110", "110")
	sell, sellMove := repairM984Fill(t, "sell", realizedAlice, realizedAlice, 20, 1200, "100", "100", "180")
	for _, tc := range []struct {
		name  string
		moves []traderTokenTransfer
	}{
		{"transfer_out_then_unrelated_in", []traderTokenTransfer{repairM984Movement(12, realizedAlice, realizedBob, "100"), repairM984Movement(14, realizedCarol, realizedAlice, "100")}},
		{"transfer_out_only", []traderTokenTransfer{repairM984Movement(12, realizedAlice, realizedBob, "60")}},
		{"unrelated_transfer_in_only", []traderTokenTransfer{repairM984Movement(14, realizedCarol, realizedAlice, "100")}},
		{"unknown_inventory_before_buy", []traderTokenTransfer{repairM984Movement(8, realizedCarol, realizedAlice, "100")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			movements := append([]traderTokenTransfer{sellMove, buyMove}, tc.moves...)
			snapshot, excluded := repairM984Snapshot(t, []accounting.FillV1{sell, buy}, movements, 1000, 2000)
			if excluded != 1 || len(snapshot.Entries) != 0 || snapshot.Quality != "partial" {
				t.Fatalf("unsupported realized profit: %+v excluded=%d", snapshot, excluded)
			}
			replay, again := repairM984Snapshot(t, []accounting.FillV1{buy, sell}, movements, 1000, 2000)
			if again != excluded || !reflect.DeepEqual(snapshot, replay) {
				t.Fatalf("unstable projection: %+v %+v", snapshot, replay)
			}
		})
	}
}

func TestRepairM_984_ReceiptMatchedBasisAndWindow(t *testing.T) {
	buy, buyMove := repairM984Fill(t, "buy", realizedBob, realizedAlice, 10, 900, "100", "110", "110")
	sell, sellMove := repairM984Fill(t, "sell", realizedBob, realizedAlice, 20, 1200, "50", "50", "80")
	movements := []traderTokenTransfer{sellMove, buyMove,
		repairM984Movement(11, realizedAlice, realizedAlice, "100"),
		repairM984Movement(12, realizedCarol, realizedAlice, "0"),
		repairM984Movement(22, realizedAlice, realizedCarol, "50")}
	otherAsset := repairM984Movement(13, realizedCarol, realizedAlice, "100")
	otherAsset.Token = "0x5555555555555555555555555555555555555555"
	movements = append(movements, otherAsset)
	snapshot, excluded := repairM984Snapshot(t, []accounting.FillV1{sell, buy}, movements, 1000, 2000)
	if excluded != 0 || len(snapshot.Entries) != 1 || snapshot.Quality != "complete" {
		t.Fatalf("proven position lost: %+v excluded=%d", snapshot, excluded)
	}
	entry := snapshot.Entries[0]
	if entry.Wallet != realizedAlice || entry.MetricSigned != "25" || entry.MetricRaw != "25" || entry.Quality != "complete" {
		t.Fatalf("wrong owner/basis: %+v", entry)
	}
	public := entry.PublicEntry(1)
	if public["metricSigned"] != "25" || public["wallet"] != realizedAlice || public["quality"] != "complete" {
		t.Fatal(public)
	}
	late, excluded := repairM984Snapshot(t, []accounting.FillV1{buy, sell}, movements, 1200, 2000)
	if excluded != 0 || len(late.Entries) != 0 || late.Quality != "complete" {
		t.Fatalf("window boundary: %+v %d", late, excluded)
	}
}

func TestRepairM_984_MissingOrMismatchedProvenanceIsUnknown(t *testing.T) {
	buy, buyMove := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 10, 900, "100", "110", "110")
	sell, sellMove := repairM984Fill(t, "sell", realizedAlice, realizedAlice, 20, 1200, "50", "50", "80")
	for _, tc := range []struct {
		name   string
		change func(*traderTokenTransfer)
		omit   bool
	}{
		{name: "missing_buy_settlement", omit: true},
		{name: "receipt_mismatch", change: func(m *traderTokenTransfer) { m.Provenance.ReceiptSHA256 = strings.Repeat("e", 64) }},
		{name: "amount_mismatch", change: func(m *traderTokenTransfer) { m.AmountRaw = "99" }},
		{name: "owner_mismatch", change: func(m *traderTokenTransfer) { m.To = realizedCarol }},
		{name: "settlement_after_fill", change: func(m *traderTokenTransfer) { m.Position.LogIndex = "3" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			move := buyMove
			if tc.change != nil {
				tc.change(&move)
			}
			moves := []traderTokenTransfer{sellMove}
			if !tc.omit {
				moves = append(moves, move)
			}
			snapshot, excluded := repairM984Snapshot(t, []accounting.FillV1{buy, sell}, moves, 1000, 2000)
			if excluded != 1 || len(snapshot.Entries) != 0 || snapshot.Quality != "partial" {
				t.Fatalf("unproven basis admitted: %+v %d", snapshot, excluded)
			}
		})
	}
}

func TestRepairM_984_UnknownBasisPersistsAndCountsWalletOnce(t *testing.T) {
	buy, buyMove := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 10, 900, "100", "110", "110")
	sell, sellMove := repairM984Fill(t, "sell", realizedAlice, realizedAlice, 20, 1200, "100", "100", "180")
	nextBuy, nextMove := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 25, 1500, "50", "60", "60")
	nextSell, lastMove := repairM984Fill(t, "sell", realizedAlice, realizedAlice, 30, 1800, "50", "50", "90")
	moves := []traderTokenTransfer{buyMove, sellMove, nextMove, lastMove, repairM984Movement(12, realizedAlice, realizedBob, "100"), repairM984Movement(14, realizedCarol, realizedAlice, "100")}
	fills := []accounting.FillV1{buy, sell, nextBuy, nextSell}
	snapshot, excluded := repairM984Snapshot(t, fills, moves, 1000, 2000)
	if excluded != 1 || len(snapshot.Entries) != 0 || snapshot.Quality != "partial" {
		t.Fatalf("unknown wallet counted incorrectly: %+v %d", snapshot, excluded)
	}
	later, excluded := repairM984Snapshot(t, fills, moves, 1600, 2000)
	if excluded != 1 || later.Quality != "partial" || len(later.Entries) != 0 {
		t.Fatalf("later buy restored unknown basis: %+v %d", later, excluded)
	}
	// The exclusion is part of the snapshot identity even when both rankings are empty.
	empty := traderLeaderboardSnapshot("trader", "24h", 2000, []LeaderboardEntry{}, 0, snapshot.Watermark)
	if empty.SnapshotID == snapshot.SnapshotID || empty.Quality != "complete" {
		t.Fatalf("unknown-basis evidence lost from snapshot identity: %+v", empty)
	}
}

func TestRepairM_984_MalformedProjectionIsRejected(t *testing.T) {
	buy, move := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 10, 900, "100", "110", "110")
	bad := move
	bad.AmountRaw = "-1"
	if _, err := traderProjectionRows([]accounting.FillV1{buy}, []traderTokenTransfer{bad}); err != ErrDataIntegrity {
		t.Fatalf("malformed movement: %v", err)
	}
	if _, err := traderProjectionRows([]accounting.FillV1{buy}, []traderTokenTransfer{move, move}); err != ErrDataIntegrity {
		t.Fatalf("duplicate movement: %v", err)
	}
	buy.SourceFillID = "invalid"
	if _, err := traderProjectionRows([]accounting.FillV1{buy}, nil); err != ErrDataIntegrity {
		t.Fatalf("malformed fill: %v", err)
	}
}

func TestRepairM_984_ExclusionPreservesProvenWallets(t *testing.T) {
	buy, buyMove := repairM984Fill(t, "buy", realizedAlice, realizedAlice, 10, 900, "100", "110", "110")
	sell, sellMove := repairM984Fill(t, "sell", realizedAlice, realizedAlice, 20, 1200, "100", "100", "180")
	cleanBuy, cleanBuyMove := repairM984Fill(t, "buy", realizedBob, realizedBob, 11, 950, "100", "110", "110")
	cleanSell, cleanSellMove := repairM984Fill(t, "sell", realizedBob, realizedBob, 21, 1300, "50", "50", "80")
	moves := []traderTokenTransfer{buyMove, sellMove, cleanBuyMove, cleanSellMove, repairM984Movement(12, realizedAlice, realizedCarol, "100"), repairM984Movement(14, realizedCarol, realizedAlice, "100")}
	snapshot, excluded := repairM984Snapshot(t, []accounting.FillV1{sell, cleanBuy, buy, cleanSell}, moves, 1000, 2000)
	if excluded != 1 || len(snapshot.Entries) != 1 || snapshot.Quality != "partial" {
		t.Fatalf("mixed projection: %+v %d", snapshot, excluded)
	}
	if entry := snapshot.Entries[0]; entry.Wallet != realizedBob || entry.MetricSigned != "25" || entry.Quality != "complete" {
		t.Fatalf("proven wallet corrupted: %+v", entry)
	}
}
