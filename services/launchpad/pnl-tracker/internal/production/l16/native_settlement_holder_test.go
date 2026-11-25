package l16

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNativeSettlementHolderModuleAddress(t *testing.T) {
	if launchpadModuleAddress != "0x7b3937a5f4397ec652546ef2e72a9e9a35d5ac52" {
		t.Fatal("launchpad module address differs from the chain address")
	}
	raw, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "services", "launchpad", "indexer", "internal", "receipt", "testdata", "producer_receipts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		NativeSwap common.Hash
		Responses  map[string]struct{ Result json.RawMessage }
	}
	if err = json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal([]common.Hash{packet.NativeSwap})
	if err != nil {
		t.Fatal(err)
	}
	var receipt types.Receipt
	if err = json.Unmarshal(packet.Responses["eth_getTransactionReceipt "+string(params)].Result, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Logs) == 0 || receipt.Logs[0] == nil {
		t.Fatal("recorded settlement transfer absent")
	}
	transfer := receipt.Logs[0]
	if transfer.Address != common.HexToAddress(usdlAddress) || len(transfer.Topics) != 3 || transfer.Topics[0] != transferTopic || common.BytesToAddress(transfer.Topics[2].Bytes()) != common.HexToAddress(launchpadModuleAddress) {
		t.Fatal("recorded settlement transfer differs from the module address")
	}
}

type nativeSettlementFixture struct {
	service                *Service
	pool                   *pgxpool.Pool
	origin                 string
	fill                   accounting.FillV1
	receiptRaw             []byte
	tokenIndex, quoteIndex int
}

func newNativeSettlementFixture(t *testing.T, ctx context.Context) nativeSettlementFixture {
	t.Helper()
	s, pool := recordedRebuildService(t, ctx)
	fixture := nativeSettlementFixture{service: s, pool: pool, origin: coordinatorReplayRecordedRPC(t), tokenIndex: -1, quoteIndex: -1}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 WHERE payload->>'side'='buy' ORDER BY log_index LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var err error
	fixture.fill, err = accounting.DecodeFillV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.fill.Side != sideBuy {
		t.Fatal("recorded first fill is not a buy")
	}
	receipt, err := s.authority.rpc.TransactionReceipt(ctx, common.HexToHash(fixture.fill.Position.TransactionHash))
	if err != nil {
		t.Fatal(err)
	}
	for i, log := range receipt.Logs {
		if len(log.Topics) != 3 || log.Topics[0] != transferTopic {
			continue
		}
		if log.Address == common.HexToAddress(fixture.fill.Token.Address) {
			fixture.tokenIndex = i
		}
		if log.Address == common.HexToAddress(fixture.fill.Settlement.Address) && new(big.Int).SetBytes(log.Data).String() == fixture.fill.USDLAmountRaw {
			fixture.quoteIndex = i
		}
	}
	if fixture.tokenIndex < 0 || fixture.quoteIndex < 0 {
		t.Fatal("recorded settlement transfers absent")
	}
	fixture.receiptRaw, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f nativeSettlementFixture) nativeBuy(t *testing.T) (accounting.FillV1, *types.Receipt) {
	t.Helper()
	var receipt types.Receipt
	if err := json.Unmarshal(f.receiptRaw, &receipt); err != nil {
		t.Fatal(err)
	}
	fill := f.fill
	fill.ActualEmitter = nativeExecutionAddress
	for _, log := range receipt.Logs {
		if log.Address == common.HexToAddress(f.fill.ActualEmitter) {
			log.Address = common.HexToAddress(nativeExecutionAddress)
		}
	}
	receipt.Logs[f.quoteIndex].Topics[2] = common.HexToHash(launchpadModuleAddress)
	return fill, &receipt
}

func (f nativeSettlementFixture) bind(t *testing.T, ctx context.Context, tx pgx.Tx, fill accounting.FillV1, receipt *types.Receipt) (*BaselineAuthority, accounting.FillV1) {
	t.Helper()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	fill.Provenance.ReceiptSHA256 = delivery.Hash(raw)
	fill.Attribution.Evidence = append([]accounting.EvidenceRef(nil), fill.Attribution.Evidence...)
	for i := range fill.Attribution.Evidence {
		if fill.Attribution.Evidence[i].Kind == "input-transfer" || fill.Attribution.Evidence[i].Kind == "output-transfer" {
			fill.Attribution.Evidence[i].ArtifactSHA256 = fill.Provenance.ReceiptSHA256
		}
	}
	fillRaw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$1,source_fact_digest=$2 WHERE source_fill_id=$3`, fillRaw, string(digest), fill.SourceFillID); err != nil {
		t.Fatal(err)
	}
	var envelope any
	if err = json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	client, _ := inventoryProviderVariant(t, f.origin, func(r inventoryRPCRequest) bool {
		if r.Method != "eth_getTransactionReceipt" || len(r.Params) != 1 {
			return false
		}
		var hash string
		return json.Unmarshal(r.Params[0], &hash) == nil && hash == fill.Position.TransactionHash
	}, func(out map[string]any) { out["result"] = envelope })
	a := *f.service.authority
	a.rpc = client
	return &a, fill
}

func TestNativeSettlementHolderLookups(t *testing.T) {
	ctx := context.Background()
	fixture := newNativeSettlementFixture(t, ctx)
	for _, tc := range []struct {
		name                   string
		sell, ordinary         bool
		holder                 string
		amountDelta, duplicate bool
		want                   error
	}{
		{name: "native buy", holder: launchpadModuleAddress},
		{name: "native buy at pool", holder: fixture.fill.PoolAddress, want: ErrCostAllocationUnavailable},
		{name: "native buy at third address", holder: "0x" + strings.Repeat("e", 40), want: ErrCostAllocationUnavailable},
		{name: "native buy amount differs by one", holder: launchpadModuleAddress, amountDelta: true, want: ErrCostAllocationUnavailable},
		{name: "native buy duplicate settlement", holder: launchpadModuleAddress, duplicate: true, want: ErrCostAllocationUnavailable},
		{name: "native sell", sell: true, holder: launchpadModuleAddress},
		{name: "native sell at pool", sell: true, holder: fixture.fill.PoolAddress, want: ErrCostAllocationUnavailable},
		{name: "ordinary buy", ordinary: true, holder: fixture.fill.PoolAddress},
		{name: "ordinary buy at module", ordinary: true, holder: launchpadModuleAddress, want: ErrCostAllocationUnavailable},
		{name: "ordinary sell", sell: true, ordinary: true, holder: fixture.fill.PoolAddress},
		{name: "ordinary sell at module", sell: true, ordinary: true, holder: launchpadModuleAddress, want: ErrCostAllocationUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill, receipt := fixture.nativeBuy(t)
			token, quote := receipt.Logs[fixture.tokenIndex], receipt.Logs[fixture.quoteIndex]
			sender := fill.Attribution.TransactionSender
			quote.Topics[2] = common.HexToHash(tc.holder)
			if tc.ordinary {
				fill.ActualEmitter = fill.PoolAddress
				for _, log := range receipt.Logs {
					if log.Address == common.HexToAddress(nativeExecutionAddress) {
						log.Address = common.HexToAddress(fill.ActualEmitter)
					}
				}
			}
			if tc.sell {
				fill.Side = sideSell
				fill.PaidInputRaw = fill.TokenAmountRaw
				fill.FeeDisposition = "retained_token_reserve"
				token.Topics[1], token.Topics[2] = common.HexToHash(sender), common.HexToHash(fill.PoolAddress)
				quote.Topics[1], quote.Topics[2] = common.HexToHash(tc.holder), common.HexToHash(sender)
			}
			if tc.amountDelta {
				quote.Data = new(big.Int).Add(new(big.Int).SetBytes(quote.Data), big.NewInt(1)).FillBytes(make([]byte, 32))
			}
			if tc.duplicate {
				copy := *quote
				copy.Index = token.Index
				receipt.Logs = append(receipt.Logs, &copy)
			}
			for _, live := range []bool{false, true} {
				t.Run(map[bool]string{false: "single fill", true: "receipt page"}[live], func(t *testing.T) {
					tx, err := fixture.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer inventoryRollback(tx)
					a, bound := fixture.bind(t, ctx, tx, fill, receipt)
					var settlement receiptSettlement
					if live {
						settlement, err = a.liveReceiptSettlement(ctx, tx, bound)
					} else {
						settlement, err = a.receiptSettlement(ctx, tx, bound)
					}
					if !errors.Is(err, tc.want) {
						t.Fatalf("settlement error = %v, want %v", err, tc.want)
					}
					if tc.want != nil {
						return
					}
					movement, err := decodeTransfer(*token)
					if err != nil {
						t.Fatal(err)
					}
					if settlement.payer != sender || settlement.recipient != sender || settlement.tokenMovement != movement || settlement.digest != bound.Provenance.ReceiptSHA256 || settlement.fill.SourceFillID != bound.SourceFillID {
						t.Fatalf("settlement = %+v, want sender %s and token movement %+v", settlement, sender, movement)
					}
				})
			}
		})
	}
}

func TestNativeSettlementHolderLiveRebuild(t *testing.T) {
	ctx := context.Background()
	fixture := newNativeSettlementFixture(t, ctx)
	fill, receipt := fixture.nativeBuy(t)
	sender := fill.Attribution.TransactionSender
	inputLog := strconv.FormatUint(uint64(receipt.Logs[fixture.quoteIndex].Index), 10)
	outputLog := strconv.FormatUint(uint64(receipt.Logs[fixture.tokenIndex].Index), 10)
	var calldataEvidence accounting.EvidenceRef
	for _, evidence := range fill.Attribution.Evidence {
		if evidence.Kind == "router-calldata" {
			calldataEvidence = evidence
		}
	}
	if calldataEvidence.Kind != "router-calldata" {
		t.Fatal("recorded router calldata evidence absent")
	}
	fill.Attribution = accounting.Attribution{
		TransactionSender: sender, RouterCaller: &sender, Trader: &sender, Payer: &sender, Recipient: &sender, Quality: "qualified_calldata",
		Evidence: []accounting.EvidenceRef{
			calldataEvidence,
			{Kind: "input-transfer", ArtifactID: fill.Position.TransactionHash, LogIndex: &inputLog},
			{Kind: "output-transfer", ArtifactID: fill.Position.TransactionHash, LogIndex: &outputLog},
		},
	}
	tx, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	if _, err = tx.Exec(ctx, `SET LOCAL search_path TO pnl,pg_catalog`); err != nil {
		t.Fatal(err)
	}
	authority, bound := fixture.bind(t, ctx, tx, fill, receipt)
	service := *fixture.service
	service.authority = authority
	height, err := strconv.ParseInt(fill.Position.Height, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.liveRebuildTx(ctx, tx, sender, fill.Token.Address, height)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Height != height || snapshot.State.Known != fill.TokenAmountRaw || snapshot.State.Unknown != "0" || snapshot.State.Cost != fill.USDLAmountRaw {
		t.Fatalf("native buy position = %+v, want quantity %s and cost %s", snapshot, fill.TokenAmountRaw, fill.USDLAmountRaw)
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT payload FROM pnl.inventory_l16_archive_v2 WHERE partition=$1`, snapshot.Partition).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var record EconomicRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record.SourceFillID != bound.SourceFillID || record.SourceDigest != bound.Provenance.ReceiptSHA256 || record.Leg.Kind != sideBuy || record.Payer != sender || record.Recipient != sender {
		t.Fatalf("native buy archive = %+v", record)
	}
}
